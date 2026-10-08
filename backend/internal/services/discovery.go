package services

import (
	"context"
	"math/rand/v2"
	"sort"
	"strings"
	"time"

	"atish/internal/apperr"
	"atish/internal/models"
	"atish/internal/repository"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type DiscoveryService struct {
	repo      *repository.Repo
	settings  *Settings
	billing   *BillingService
	presenter *Presenter
	notifier  *Notifier
	rdb       *redis.Client
}

func NewDiscovery(repo *repository.Repo, settings *Settings, billing *BillingService, presenter *Presenter, notifier *Notifier, rdb *redis.Client) *DiscoveryService {
	return &DiscoveryService{repo: repo, settings: settings, billing: billing, presenter: presenter, notifier: notifier, rdb: rdb}
}

type FeedFilter struct {
	Type      string
	AgeMin    int
	AgeMax    int
	City      string
	Interests []string
	Language  string
	Limit     int
}

type FeedResponse struct {
	Profiles  []*models.PublicProfile `json:"profiles"`
	LikesLeft *int                    `json:"likes_left"` // nil = unlimited
}

func (s *DiscoveryService) Feed(ctx context.Context, uid uuid.UUID, f FeedFilter) (*FeedResponse, error) {
	me, err := s.repo.LoadProfile(ctx, uid)
	if err != nil {
		return nil, err
	}
	if me.OnboardedAt == nil || me.Location == nil {
		return nil, apperr.Forbidden("onboarding_required", "Finish setting up your profile first")
	}
	batch := s.settings.Int(ctx, "discovery_batch_size", 10)
	if f.Limit <= 0 || f.Limit > 30 {
		f.Limit = batch
	}
	cf := repository.CandidateFilter{
		MeID: uid, Type: f.Type, AgeMin: me.Preferences.AgeMin, AgeMax: me.Preferences.AgeMax,
		LocationCity: strings.ToLower(f.City), Interests: f.Interests, Language: f.Language,
		ResurfaceDays: s.settings.Int(ctx, "pass_resurface_days", 30), Limit: 300,
	}
	if f.AgeMin > 0 {
		cf.AgeMin = max(f.AgeMin, cf.AgeMin)
	}
	if f.AgeMax > 0 {
		cf.AgeMax = min(f.AgeMax, cf.AgeMax)
	}
	if me.Preferences.DistanceScope != "anywhere" {
		cf.Country = me.Location.CountryCode
	}
	cands, err := s.repo.Candidates(ctx, cf)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, len(cands))
	likedMe := map[uuid.UUID]bool{}
	for i, c := range cands {
		ids[i], likedMe[c.ID] = c.ID, c.LikedMe
	}
	profiles, err := s.repo.LoadProfiles(ctx, ids)
	if err != nil {
		return nil, err
	}
	w := s.settings.Weights(ctx)

	type scored struct {
		p *models.Profile
		r CompatResult
		s float64
	}
	var list []scored
	for _, id := range ids {
		p := profiles[id]
		if p == nil || len(p.Photos) == 0 {
			continue
		}
		r := Evaluate(me, p, w, likedMe[id])
		if !r.Compatible {
			continue
		}
		if f.Type != "" && !contains(r.Types, f.Type) {
			continue
		}
		list = append(list, scored{p, r, r.Score + rand.Float64()*4}) // jitter keeps the deck fresh
	}
	sort.Slice(list, func(i, j int) bool { return list[i].s > list[j].s })
	if len(list) > f.Limit {
		list = list[:f.Limit]
	}
	out := &FeedResponse{Profiles: make([]*models.PublicProfile, 0, len(list))}
	for _, x := range list {
		c := x.r.Compat
		out.Profiles = append(out.Profiles, s.presenter.Public(x.p, &c, likedMe[x.p.ID]))
	}
	out.LikesLeft = s.likesLeft(ctx, uid)
	return out, nil
}

func contains(l []string, v string) bool {
	for _, x := range l {
		if x == v {
			return true
		}
	}
	return false
}

func dayKey(uid uuid.UUID) string {
	return "likes:" + uid.String() + ":" + time.Now().UTC().Format("20060102")
}

func (s *DiscoveryService) likesLeft(ctx context.Context, uid uuid.UUID) *int {
	if s.billing.Has(ctx, uid, "unlimited_likes") {
		return nil
	}
	limit := s.settings.Int(ctx, "free_daily_likes", 60)
	used, _ := s.rdb.Get(ctx, dayKey(uid)).Int()
	left := max(limit-used, 0)
	return &left
}

type SwipeResult struct {
	Matched   bool                  `json:"matched"`
	MatchID   *uuid.UUID            `json:"match_id,omitempty"`
	User      *models.PublicProfile `json:"user,omitempty"`
	LikesLeft *int                  `json:"likes_left"`
}

// eligibleTarget loads both profiles and confirms `me` may interact with `target`.
// Ineligible targets look like "not found" so ids can't be probed.
func (s *DiscoveryService) eligibleTarget(ctx context.Context, me, target uuid.UUID) (*models.Profile, *models.Profile, CompatResult, error) {
	if me == target {
		return nil, nil, CompatResult{}, apperr.NotFound("user not found")
	}
	ps, err := s.repo.LoadProfiles(ctx, []uuid.UUID{me, target})
	if err != nil {
		return nil, nil, CompatResult{}, err
	}
	a, b := ps[me], ps[target]
	if a == nil || b == nil || b.Status != models.StatusActive || b.OnboardedAt == nil || !b.Privacy.Discoverable {
		return nil, nil, CompatResult{}, apperr.NotFound("user not found")
	}
	if blocked, err := s.repo.IsBlockedEither(ctx, me, target); err != nil || blocked {
		return nil, nil, CompatResult{}, apperr.NotFound("user not found")
	}
	r := Evaluate(a, b, s.settings.Weights(ctx), false)
	if !r.Compatible {
		return nil, nil, r, apperr.Forbidden("not_compatible", "You two are looking for different things")
	}
	return a, b, r, nil
}

func (s *DiscoveryService) Like(ctx context.Context, me, target uuid.UUID) (*SwipeResult, error) {
	a, b, r, err := s.eligibleTarget(ctx, me, target)
	if err != nil {
		return nil, err
	}
	// Daily limit for free users (re-liking someone doesn't consume the quota).
	if prev := s.repo.SwipeAction(ctx, me, target); prev != "like" && !s.billing.Has(ctx, me, "unlimited_likes") {
		limit := s.settings.Int(ctx, "free_daily_likes", 60)
		key := dayKey(me)
		n, err := s.rdb.Incr(ctx, key).Result()
		if err == nil {
			if n == 1 {
				s.rdb.Expire(ctx, key, 48*time.Hour)
			}
			if int(n) > limit {
				s.rdb.Decr(ctx, key)
				return nil, apperr.PaymentRequired("daily_limit", "You've used all your likes for today").With("limit", limit)
			}
		}
	}
	m, err := s.repo.Swipe(ctx, me, target, "like", r.Types)
	if err != nil {
		return nil, err
	}
	res := &SwipeResult{LikesLeft: s.likesLeft(ctx, me)}
	if m != nil {
		res.Matched = true
		res.MatchID = &m.ID
		c := r.Compat
		res.User = s.presenter.Public(b, &c, true)
		s.notifier.NewMatch(ctx, target, a.DisplayName, m.ID)
	}
	return res, nil
}

func (s *DiscoveryService) Pass(ctx context.Context, me, target uuid.UUID) error {
	if me == target {
		return apperr.NotFound("user not found")
	}
	if _, err := s.repo.GetUser(ctx, target); err != nil {
		return err
	}
	_, err := s.repo.Swipe(ctx, me, target, "pass", nil)
	return err
}

// Rewind undoes the most recent pass (premium).
func (s *DiscoveryService) Rewind(ctx context.Context, me uuid.UUID) (*models.PublicProfile, error) {
	if !s.billing.Has(ctx, me, "rewind") {
		return nil, apperr.PaymentRequired("premium_required", "Rewind is a premium feature")
	}
	id, err := s.repo.RewindLastPass(ctx, me)
	if err != nil {
		return nil, err
	}
	a, b, r, err := s.eligibleTarget(ctx, me, id)
	if err != nil {
		return nil, err
	}
	_ = a
	c := r.Compat
	return s.presenter.Public(b, &c, false), nil
}

type LikesReceived struct {
	Total    int                     `json:"total"`
	Locked   bool                    `json:"locked"`
	Profiles []*models.PublicProfile `json:"profiles"`
}

func (s *DiscoveryService) LikesReceived(ctx context.Context, me uuid.UUID) (*LikesReceived, error) {
	ids, total, err := s.repo.LikesReceived(ctx, me, 50)
	if err != nil {
		return nil, err
	}
	out := &LikesReceived{Total: total, Profiles: []*models.PublicProfile{}}
	if !s.billing.Has(ctx, me, "see_likes") {
		out.Locked = true
		return out, nil
	}
	ps, err := s.repo.LoadProfiles(ctx, append([]uuid.UUID{me}, ids...))
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		if p := ps[id]; p != nil && len(p.Photos) > 0 {
			out.Profiles = append(out.Profiles, s.presenter.Public(p, nil, true))
		}
	}
	return out, nil
}

// ---- matches ----

func (s *DiscoveryService) Matches(ctx context.Context, me uuid.UUID) ([]models.Match, error) {
	rows, err := s.repo.ListMatches(ctx, me)
	if err != nil {
		return nil, err
	}
	return s.hydrate(ctx, me, rows)
}

func (s *DiscoveryService) hydrate(ctx context.Context, me uuid.UUID, rows []repository.MatchRow) ([]models.Match, error) {
	ids := make([]uuid.UUID, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.Other(me))
	}
	ps, err := s.repo.LoadProfiles(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]models.Match, 0, len(rows))
	for _, r := range rows {
		p := ps[r.Other(me)]
		if p == nil {
			continue
		}
		out = append(out, models.Match{ID: r.ID, User: s.presenter.Public(p, nil, false), ConnectionTypes: r.Types,
			CreatedAt: r.CreatedAt, LastMessageAt: r.LastMessageAt})
	}
	return out, nil
}

func (s *DiscoveryService) Unmatch(ctx context.Context, me, matchID uuid.UUID) error {
	return s.repo.Unmatch(ctx, matchID, me)
}

// MatchProfile returns the full public profile of a matched user.
func (s *DiscoveryService) MatchProfile(ctx context.Context, me, other uuid.UUID) (*models.PublicProfile, error) {
	m, err := s.repo.GetMatchBetween(ctx, me, other)
	if err != nil || m.UnmatchedAt != nil {
		return nil, apperr.NotFound("user not found")
	}
	ps, err := s.repo.LoadProfiles(ctx, []uuid.UUID{other})
	if err != nil || ps[other] == nil || ps[other].Status != models.StatusActive {
		return nil, apperr.NotFound("user not found")
	}
	return s.presenter.Public(ps[other], nil, false), nil
}
