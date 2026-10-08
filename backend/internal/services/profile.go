package services

import (
	"bytes"
	"context"
	"io"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"atish/internal/apperr"
	"atish/internal/config"
	"atish/internal/models"
	"atish/internal/repository"
	"atish/internal/storage"
	"atish/internal/util"

	"github.com/google/uuid"
)

type ProfileService struct {
	cfg      *config.Config
	repo     *repository.Repo
	auth     *AuthService
	settings *Settings
	catalog  *CatalogService
	store    storage.Storage
	signer   *MediaSigner
	billing  *BillingService
}

func NewProfile(cfg *config.Config, repo *repository.Repo, auth *AuthService, settings *Settings, catalog *CatalogService,
	store storage.Storage, signer *MediaSigner, billing *BillingService) *ProfileService {
	return &ProfileService{cfg: cfg, repo: repo, auth: auth, settings: settings, catalog: catalog, store: store, signer: signer, billing: billing}
}

const MinInterestsForOnboarding = 3
const MaxInterests = 15

var (
	genders       = []string{"male", "female", "non_binary", "other", "prefer_not_to_say"}
	levels        = []string{"basic", "intermediate", "advanced", "native"}
	intentions    = []string{"long_term", "short_term", "casual", "getting_to_know", "not_sure"}
	prefGenders   = []string{"men", "women", "non_binary", "everyone", "prefer_not_to_say"}
	scopes        = []string{"same_area", "same_city", "nearby", "same_region", "same_country", "anywhere"}
	slots         = []string{"morning", "afternoon", "evening", "late_night"}
	occStatuses   = []string{"student", "employed", "entrepreneur", "other"}
	lifestyleSets = map[string][]string{
		"smoking":  {"never", "sometimes", "regularly"},
		"drinking": {"never", "socially", "regularly"},
		"pets":     {"have", "want", "none", "allergic"},
		"diet":     {"omnivore", "vegetarian", "vegan", "other"},
		"sleep":    {"early", "night", "flexible"},
		"social":   {"introvert", "ambivert", "extrovert"},
	}
	countryRe = regexp.MustCompile(`^[A-Z]{2}$`)
)

// ---- aggregate for "me" ----

type MeResponse struct {
	*models.Profile
	Onboarding   OnboardingStatus    `json:"onboarding"`
	Completeness int                 `json:"completeness"`
	Suggestions  []string            `json:"suggestions"`
	Entitlements models.Entitlements `json:"entitlements"`
	Settings     map[string]any      `json:"app"`
}

type OnboardingStatus struct {
	Complete bool     `json:"complete"`
	Missing  []string `json:"missing"`
}

func (s *ProfileService) Me(ctx context.Context, uid uuid.UUID) (*MeResponse, error) {
	p, err := s.repo.LoadProfile(ctx, uid)
	if err != nil {
		return nil, err
	}
	s.signPhotos(p.Photos)
	ent := s.billing.Entitlements(ctx, uid)
	p.Premium = ent.Premium
	missing := s.missingForOnboarding(p)
	comp, sugg := completeness(p)
	return &MeResponse{
		Profile:      p,
		Onboarding:   OnboardingStatus{Complete: p.OnboardedAt != nil, Missing: missing},
		Completeness: comp,
		Suggestions:  sugg,
		Entitlements: ent,
		Settings:     s.settings.Public(ctx),
	}, nil
}

func (s *ProfileService) signPhotos(photos []models.Photo) {
	for i := range photos {
		photos[i].URL = s.signer.URL(photos[i].ID)
	}
}

func (s *ProfileService) missingForOnboarding(p *models.Profile) []string {
	m := []string{}
	if p.AtishUsername == nil {
		m = append(m, "username")
	}
	if !p.HasProfile {
		m = append(m, "basics")
	}
	if len(p.Photos) == 0 {
		m = append(m, "photo")
	}
	if p.Location == nil {
		m = append(m, "location")
	}
	if len(p.ConnectionTypes) == 0 {
		m = append(m, "connection_types")
	}
	if len(p.Interests) < MinInterestsForOnboarding {
		m = append(m, "interests")
	}
	return m
}

func completeness(p *models.Profile) (int, []string) {
	score, sugg := 0, []string{}
	add := func(ok bool, pts int, key string) {
		if ok {
			score += pts
		} else {
			sugg = append(sugg, key)
		}
	}
	add(len(p.Photos) > 0, 15, "photo")
	add(len(p.Photos) > 1, 5, "more_photos")
	add(len(p.Bio) >= 20, 10, "bio")
	add(len(p.Interests) >= 5, 15, "interests")
	add(len(p.Languages) > 0, 10, "languages")
	add(len(p.FriendshipKinds) > 0 || p.Preferences.RelationshipIntention != "", 10, "preferences")
	add(len(p.Personality) >= 5, 15, "personality")
	add(len(p.Availability) > 0, 5, "availability")
	add(p.Occupation.Status != "" || p.Occupation.Profession != "", 5, "work")
	add(p.PhoneVerified, 10, "phone")
	return score, sugg
}

// ---- profile updates ----

func (s *ProfileService) UpdateProfile(ctx context.Context, uid uuid.UUID, patch models.ProfilePatch) error {
	cur, err := s.repo.LoadProfile(ctx, uid)
	if err != nil {
		return err
	}
	cat, err := s.catalog.Get(ctx)
	if err != nil {
		return err
	}
	blocked := s.settings.Strings(ctx, "blocked_words")
	minAge := s.settings.Int(ctx, "min_age", 18)

	if patch.DisplayName != nil {
		n, ok := util.CleanText(*patch.DisplayName, 40)
		if !ok || len([]rune(n)) < 2 {
			return apperr.BadRequest("invalid_name", "Name must be 2–40 characters")
		}
		if containsWord(n, blocked) {
			return apperr.BadRequest("name_not_allowed", "That name is not allowed")
		}
		patch.DisplayName = &n
	}
	var birth *time.Time
	if patch.BirthDate != nil {
		t, err := time.Parse("2006-01-02", *patch.BirthDate)
		if err != nil {
			return apperr.BadRequest("invalid_birth_date", "Invalid date of birth")
		}
		age := util.AgeOn(t, time.Now())
		if age < minAge {
			return apperr.BadRequest("too_young", "You must be at least "+strconv.Itoa(minAge)+" to use Atish").With("min_age", minAge)
		}
		if age > 100 {
			return apperr.BadRequest("invalid_birth_date", "Invalid date of birth")
		}
		birth = &t
	}
	if patch.Gender != nil && !util.Contains(genders, *patch.Gender) {
		return apperr.BadRequest("invalid_gender", "Invalid gender")
	}
	if patch.Bio != nil {
		b, ok := util.CleanText(*patch.Bio, 300)
		if !ok {
			return apperr.BadRequest("invalid_bio", "Bio can be at most 300 characters")
		}
		if containsWord(b, blocked) {
			return apperr.BadRequest("bio_not_allowed", "Your bio contains words that are not allowed")
		}
		patch.Bio = &b
	}
	if patch.LocationID != nil {
		if _, err := s.repo.GetLocation(ctx, *patch.LocationID); err != nil {
			return apperr.BadRequest("invalid_location", "Unknown location")
		}
	}
	if o := patch.Occupation; o != nil {
		if o.Status != "" && !util.Contains(occStatuses, o.Status) {
			return apperr.BadRequest("invalid_occupation", "Invalid occupation status")
		}
		for _, f := range []*string{&o.University, &o.FieldOfStudy, &o.Degree, &o.Profession, &o.Industry} {
			v, ok := util.CleanText(*f, 80)
			if !ok {
				return apperr.BadRequest("invalid_text", "Education / work fields can be at most 80 characters")
			}
			*f = v
		}
	}
	if l := patch.Lifestyle; l != nil {
		checks := []struct{ set, v string }{{"smoking", l.Smoking}, {"drinking", l.Drinking}, {"pets", l.Pets}, {"diet", l.Diet}, {"sleep", l.SleepSchedule}, {"social", l.SocialLevel}}
		for _, c := range checks {
			if c.v != "" && !util.Contains(lifestyleSets[c.set], c.v) {
				return apperr.BadRequest("invalid_lifestyle", "Invalid lifestyle value")
			}
		}
	}
	if patch.Interests != nil {
		in := util.Dedup(*patch.Interests)
		if len(in) > MaxInterests {
			return apperr.BadRequest("too_many_interests", "Pick up to 15 interests")
		}
		valid := map[string]bool{}
		for _, i := range cat.Interests {
			valid[i.Slug] = true
		}
		for _, sl := range in {
			if !valid[sl] {
				return apperr.BadRequest("invalid_interest", "Unknown interest: "+sl)
			}
		}
		patch.Interests = &in
	}
	if patch.ConnectionTypes != nil {
		in := util.Dedup(*patch.ConnectionTypes)
		if len(in) == 0 {
			return apperr.BadRequest("connection_required", "Choose what you are looking for")
		}
		valid := map[string]bool{}
		for _, c := range cat.ConnectionTypes {
			valid[c.Slug] = true
		}
		for _, sl := range in {
			if !valid[sl] {
				return apperr.BadRequest("invalid_connection_type", "Unknown connection type")
			}
		}
		patch.ConnectionTypes = &in
	}
	if patch.Languages != nil {
		if len(*patch.Languages) > 8 {
			return apperr.BadRequest("too_many_languages", "Up to 8 languages")
		}
		valid := map[string]bool{}
		for _, l := range cat.Languages {
			valid[l.Code] = true
		}
		seen := map[string]bool{}
		out := []models.UserLanguage{}
		for _, l := range *patch.Languages {
			if !valid[l.Code] || !util.Contains(levels, l.Level) {
				return apperr.BadRequest("invalid_language", "Invalid language or level")
			}
			if !seen[l.Code] {
				seen[l.Code] = true
				out = append(out, l)
			}
		}
		patch.Languages = &out
	}

	if !cur.HasProfile {
		if patch.DisplayName == nil || birth == nil || patch.Gender == nil {
			return apperr.BadRequest("basics_required", "Name, date of birth and gender are required")
		}
		if err := s.repo.CreateProfile(ctx, uid, *patch.DisplayName, *birth, *patch.Gender); err != nil {
			return err
		}
	}
	return s.repo.ApplyProfilePatch(ctx, uid, patch, birth)
}

func (s *ProfileService) UpdatePreferences(ctx context.Context, uid uuid.UUID, patch models.PreferencesPatch) error {
	cur, err := s.repo.LoadProfile(ctx, uid)
	if err != nil {
		return err
	}
	if !cur.HasProfile {
		return apperr.BadRequest("basics_required", "Create your profile first")
	}
	cat, err := s.catalog.Get(ctx)
	if err != nil {
		return err
	}
	minAge := s.settings.Int(ctx, "min_age", 18)

	if patch.FriendshipKinds != nil {
		in := util.Dedup(*patch.FriendshipKinds)
		valid := map[string]bool{}
		for _, k := range cat.FriendshipKinds {
			valid[k.Slug] = true
		}
		for _, k := range in {
			if !valid[k] {
				return apperr.BadRequest("invalid_friendship_kind", "Unknown friendship type")
			}
		}
		patch.FriendshipKinds = &in
	}
	if pr := patch.Preferences; pr != nil {
		if pr.RelationshipIntention != "" && !util.Contains(intentions, pr.RelationshipIntention) {
			return apperr.BadRequest("invalid_intention", "Invalid relationship intention")
		}
		if len(pr.PreferredGenders) == 0 {
			pr.PreferredGenders = []string{"everyone"}
		}
		for _, g := range pr.PreferredGenders {
			if !util.Contains(prefGenders, g) {
				return apperr.BadRequest("invalid_gender_pref", "Invalid gender preference")
			}
		}
		pr.PreferredGenders = util.Dedup(pr.PreferredGenders)
		if pr.AgeMin < minAge {
			pr.AgeMin = minAge
		}
		if pr.AgeMax > 99 {
			pr.AgeMax = 99
		}
		if pr.AgeMin > pr.AgeMax {
			return apperr.BadRequest("invalid_age_range", "Minimum age must not exceed maximum age")
		}
		if !util.Contains(scopes, pr.DistanceScope) {
			return apperr.BadRequest("invalid_distance", "Invalid distance preference")
		}
	}
	if patch.Personality != nil {
		opts := map[string]map[string]bool{}
		for _, q := range cat.Questions {
			opts[q.Key] = map[string]bool{}
			for _, o := range q.Options {
				opts[q.Key][o.Key] = true
			}
		}
		for k, v := range *patch.Personality {
			if !opts[k][v] {
				return apperr.BadRequest("invalid_answer", "Invalid personality answer")
			}
		}
	}
	if patch.Availability != nil {
		for _, sl := range *patch.Availability {
			if sl.Day < 0 || sl.Day > 6 || !util.Contains(slots, sl.Slot) {
				return apperr.BadRequest("invalid_availability", "Invalid availability")
			}
		}
	}
	return s.repo.ApplyPreferencesPatch(ctx, uid, patch)
}

func (s *ProfileService) SetUsername(ctx context.Context, uid uuid.UUID, suffix string) error {
	u, err := s.repo.GetUser(ctx, uid)
	if err != nil {
		return err
	}
	if err := s.auth.CheckUsername(ctx, suffix, u.AtishUsername); err != nil {
		return err
	}
	return s.repo.SetUsername(ctx, uid, UsernamePrefix+suffix)
}

// CompleteOnboarding verifies the minimum profile exists, then opens discovery.
func (s *ProfileService) CompleteOnboarding(ctx context.Context, uid uuid.UUID) error {
	p, err := s.repo.LoadProfile(ctx, uid)
	if err != nil {
		return err
	}
	if m := s.missingForOnboarding(p); len(m) > 0 {
		return apperr.BadRequest("onboarding_incomplete", "Some required steps are missing").With("missing", m)
	}
	return s.repo.MarkOnboarded(ctx, uid)
}

// ---- photos ----

func (s *ProfileService) AddPhoto(ctx context.Context, uid uuid.UUID, r io.Reader) (*models.Photo, error) {
	data, err := ProcessImage(r)
	if err != nil {
		return nil, err
	}
	return s.savePhoto(ctx, uid, data)
}

func (s *ProfileService) ImportTelegramPhoto(ctx context.Context, uid uuid.UUID) (*models.Photo, error) {
	u, err := s.repo.GetUser(ctx, uid)
	if err != nil {
		return nil, err
	}
	if u.TelegramPhotoURL == "" {
		return nil, apperr.BadRequest("no_telegram_photo", "No Telegram profile photo available")
	}
	raw, err := FetchTelegramPhoto(ctx, u.TelegramPhotoURL)
	if err != nil {
		return nil, apperr.BadRequest("telegram_photo_failed", "Could not import your Telegram photo")
	}
	data, err := ProcessImage(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	return s.savePhoto(ctx, uid, data)
}

func (s *ProfileService) savePhoto(ctx context.Context, uid uuid.UUID, data []byte) (*models.Photo, error) {
	n, err := s.repo.CountPhotos(ctx, uid)
	if err != nil {
		return nil, err
	}
	if n >= s.settings.Int(ctx, "max_photos", 3) {
		return nil, apperr.BadRequest("photo_limit", "You reached the maximum number of photos")
	}
	id := uuid.New()
	key := PhotoKey(uid, id)
	if err := s.store.Put(ctx, key, data); err != nil {
		return nil, err
	}
	if err := s.repo.AddPhoto(ctx, id, uid, key); err != nil {
		_ = s.store.Delete(ctx, key)
		return nil, err
	}
	return &models.Photo{ID: id, Position: n, URL: s.signer.URL(id)}, nil
}

func (s *ProfileService) DeletePhoto(ctx context.Context, uid, photoID uuid.UUID) error {
	key, err := s.repo.RemovePhoto(ctx, photoID, uid)
	if err != nil {
		return err
	}
	if err := s.store.Delete(ctx, key); err != nil {
		log.Printf("delete photo file %s: %v", key, err)
	}
	return nil
}

func (s *ProfileService) ReorderPhotos(ctx context.Context, uid uuid.UUID, ids []uuid.UUID) error {
	return s.repo.ReorderPhotos(ctx, uid, ids)
}

// ServePhoto resolves a signed media request into bytes.
func (s *ProfileService) ServePhoto(ctx context.Context, id uuid.UUID, exp, sig string) ([]byte, error) {
	if !s.signer.Verify(id, exp, sig) {
		return nil, apperr.Forbidden("invalid_signature", "Invalid or expired link")
	}
	key, err := s.repo.PhotoKey(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.store.Get(ctx, key)
}

// ---- locations ----

func (s *ProfileService) SearchCities(ctx context.Context, country, q string) ([]models.Location, error) {
	country = strings.ToUpper(country)
	if !countryRe.MatchString(country) {
		return nil, apperr.BadRequest("invalid_country", "Invalid country code")
	}
	q, _ = util.CleanText(q, 60)
	return s.repo.SearchCities(ctx, country, q, 12)
}

func (s *ProfileService) ListAreas(ctx context.Context, country, city string) ([]models.Location, error) {
	country = strings.ToUpper(country)
	if !countryRe.MatchString(country) {
		return nil, apperr.BadRequest("invalid_country", "Invalid country code")
	}
	return s.repo.ListAreas(ctx, country, city)
}

var placeRe = regexp.MustCompile(`^[\p{L}\p{M}0-9 .,'’\-()/]{2,60}$`)

// CreateLocation lets a user add a missing city/area. Only area-level data is
// accepted: there is no coordinate or street field anywhere in the system.
func (s *ProfileService) CreateLocation(ctx context.Context, l models.Location) (*models.Location, error) {
	l.CountryCode = strings.ToUpper(strings.TrimSpace(l.CountryCode))
	if !countryRe.MatchString(l.CountryCode) {
		return nil, apperr.BadRequest("invalid_country", "Invalid country code")
	}
	for _, f := range []*string{&l.Country, &l.Region, &l.City, &l.Area} {
		*f = strings.Join(strings.Fields(*f), " ")
	}
	if l.City == "" || !placeRe.MatchString(l.City) || (l.Region != "" && !placeRe.MatchString(l.Region)) ||
		(l.Area != "" && !placeRe.MatchString(l.Area)) || l.Country == "" || len(l.Country) > 60 {
		return nil, apperr.BadRequest("invalid_location", "Please enter a valid city and area name")
	}
	blocked := s.settings.Strings(ctx, "blocked_words")
	if containsWord(l.City+" "+l.Area+" "+l.Region, blocked) {
		return nil, apperr.BadRequest("invalid_location", "That name is not allowed")
	}
	id, err := s.repo.EnsureLocation(ctx, l)
	if err != nil {
		return nil, err
	}
	return s.repo.GetLocation(ctx, id)
}

// ---- phone verification (completed by the Telegram bot contact flow) ----

var phoneRe = regexp.MustCompile(`^\+?[0-9]{7,15}$`)

func (s *ProfileService) VerifyPhoneFromTelegram(ctx context.Context, tgID int64, phone string) error {
	phone = strings.ReplaceAll(strings.ReplaceAll(phone, " ", ""), "-", "")
	if !strings.HasPrefix(phone, "+") {
		phone = "+" + phone
	}
	if !phoneRe.MatchString(phone) {
		return apperr.BadRequest("invalid_phone", "Invalid phone number")
	}
	u, err := s.repo.GetUserByTelegramID(ctx, tgID)
	if err != nil {
		return err
	}
	enc, err := util.Encrypt(s.cfg.EncryptionKey, []byte(phone))
	if err != nil {
		return err
	}
	if err := s.repo.SavePhone(ctx, u.ID, enc, util.HashHex(s.cfg.MediaKey, phone)); err != nil {
		return err
	}
	return nil
}

func (s *ProfileService) RemovePhone(ctx context.Context, uid uuid.UUID) error {
	return s.repo.DeletePhone(ctx, uid)
}

// ---- account deletion ----

func (s *ProfileService) DeleteAccount(ctx context.Context, uid uuid.UUID) error {
	u, err := s.repo.GetUser(ctx, uid)
	if err != nil {
		return err
	}
	if u.Status == models.StatusBanned {
		return apperr.Forbidden("account_banned", "Banned accounts cannot be deleted. Contact support.")
	}
	keys, err := s.repo.DeleteAccount(ctx, uid)
	if err != nil {
		return err
	}
	for _, k := range keys {
		if err := s.store.Delete(ctx, k); err != nil {
			log.Printf("delete photo file %s: %v", k, err)
		}
	}
	s.auth.InvalidateUser(ctx, uid)
	return nil
}
