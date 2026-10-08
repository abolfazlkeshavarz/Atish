package services

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"atish/internal/apperr"
	"atish/internal/models"
	"atish/internal/repository"
	"atish/internal/storage"
	"atish/internal/telegram"
	"atish/internal/util"

	"github.com/google/uuid"
)

type AdminService struct {
	repo     *repository.Repo
	auth     *AuthService
	profile  *ProfileService
	billing  *BillingService
	settings *Settings
	catalog  *CatalogService
	bot      *telegram.Bot
	store    storage.Storage
}

func NewAdmin(repo *repository.Repo, auth *AuthService, profile *ProfileService, billing *BillingService, settings *Settings,
	catalog *CatalogService, bot *telegram.Bot, store storage.Storage) *AdminService {
	return &AdminService{repo: repo, auth: auth, profile: profile, billing: billing, settings: settings, catalog: catalog, bot: bot, store: store}
}

func (s *AdminService) Stats(ctx context.Context) (json.RawMessage, error) { return s.repo.Stats(ctx) }

func (s *AdminService) Users(ctx context.Context, q repository.UserQuery) (json.RawMessage, int, error) {
	return s.repo.AdminUsers(ctx, q)
}

type UserDetail struct {
	Profile      *models.Profile      `json:"profile"`
	Extras       json.RawMessage      `json:"extras"`
	Entitlements models.Entitlements  `json:"entitlements"`
	Subscription *models.Subscription `json:"subscription"`
	TelegramID   *int64               `json:"telegram_user_id"`
	TelegramName string               `json:"telegram_username"`
}

func (s *AdminService) UserDetail(ctx context.Context, id uuid.UUID) (*UserDetail, error) {
	p, err := s.repo.LoadProfile(ctx, id)
	if err != nil {
		return nil, err
	}
	for i := range p.Photos {
		p.Photos[i].URL = s.profile.signer.URL(p.Photos[i].ID)
	}
	extras, err := s.repo.AdminUserExtras(ctx, id)
	if err != nil {
		return nil, err
	}
	d := &UserDetail{Profile: p, Extras: extras, Entitlements: s.billing.Entitlements(ctx, id), TelegramID: p.TelegramUserID, TelegramName: p.TelegramUsername}
	if sub, err := s.repo.ActiveSubscription(ctx, id); err == nil {
		d.Subscription = sub
	}
	return d, nil
}

// UpdateUser applies moderation / verification changes. Role changes require an admin actor.
func (s *AdminService) UpdateUser(ctx context.Context, actor, actorRole string, id uuid.UUID, fields map[string]any) error {
	if _, hasRole := fields["role"]; hasRole && actorRole != models.RoleAdmin {
		return apperr.Forbidden("admin_required", "Only admins can change roles")
	}
	if st, ok := fields["status"].(string); ok {
		switch st {
		case models.StatusActive, models.StatusSuspended, models.StatusBanned:
		default:
			return apperr.BadRequest("invalid_status", "Invalid status")
		}
	}
	if r, ok := fields["role"].(string); ok && r != models.RoleUser && r != models.RoleModerator && r != models.RoleAdmin {
		return apperr.BadRequest("invalid_role", "Invalid role")
	}
	target, err := s.repo.GetUser(ctx, id)
	if err != nil {
		return err
	}
	if target.Role == models.RoleAdmin && actorRole != models.RoleAdmin {
		return apperr.Forbidden("admin_required", "Only admins can modify admin accounts")
	}
	if err := s.repo.AdminSetUser(ctx, id, fields); err != nil {
		return err
	}
	s.auth.InvalidateUser(ctx, id)
	s.repo.Audit(ctx, actor, "user.update", "user", id.String(), fields)
	return nil
}

func (s *AdminService) EditProfileText(ctx context.Context, actor string, id uuid.UUID, name, bio *string) error {
	if name != nil {
		n, ok := util.CleanText(*name, 40)
		if !ok || n == "" {
			return apperr.BadRequest("invalid_name", "Invalid name")
		}
		name = &n
	}
	if bio != nil {
		b, ok := util.CleanText(*bio, 300)
		if !ok {
			return apperr.BadRequest("invalid_bio", "Bio too long")
		}
		bio = &b
	}
	if err := s.repo.AdminEditProfileText(ctx, id, name, bio); err != nil {
		return err
	}
	s.repo.Audit(ctx, actor, "profile.edit", "user", id.String(), map[string]any{"name": name, "bio": bio})
	return nil
}

func (s *AdminService) DeleteUser(ctx context.Context, actor string, id uuid.UUID) error {
	keys, err := s.repo.DeleteAccount(ctx, id)
	if err != nil {
		return err
	}
	for _, k := range keys {
		_ = s.store.Delete(ctx, k)
	}
	s.auth.InvalidateUser(ctx, id)
	s.repo.Audit(ctx, actor, "user.delete", "user", id.String(), nil)
	return nil
}

func (s *AdminService) RemovePhoto(ctx context.Context, actor string, photoID uuid.UUID) error {
	key, err := s.repo.RemovePhoto(ctx, photoID, uuid.Nil)
	if err != nil {
		return err
	}
	_ = s.store.Delete(ctx, key)
	s.repo.Audit(ctx, actor, "photo.remove", "photo", photoID.String(), nil)
	return nil
}

func (s *AdminService) RecentPhotos(ctx context.Context, limit, offset int) (json.RawMessage, error) {
	raw, err := s.repo.AdminRecentPhotos(ctx, limit, offset)
	if err != nil {
		return nil, err
	}
	// attach short-lived signed URLs so moderators can see the images
	var rows []map[string]any
	if err := json.Unmarshal(raw, &rows); err != nil {
		return raw, nil
	}
	for _, r := range rows {
		if id, err := uuid.Parse(r["id"].(string)); err == nil {
			r["url"] = s.profile.signer.URL(id)
		}
	}
	return json.Marshal(rows)
}

func (s *AdminService) Reports(ctx context.Context, status string, limit, offset int) (json.RawMessage, error) {
	return s.repo.AdminReports(ctx, status, limit, offset)
}

type ReportDetail struct {
	Report       json.RawMessage `json:"report"`
	Conversation json.RawMessage `json:"conversation"`
}

func (s *AdminService) Report(ctx context.Context, actor string, id int64) (*ReportDetail, error) {
	r, err := s.repo.AdminReport(ctx, id)
	if err != nil {
		return nil, err
	}
	d := &ReportDetail{Report: r, Conversation: json.RawMessage("[]")}
	var head struct {
		MatchID *uuid.UUID `json:"match_id"`
	}
	_ = json.Unmarshal(r, &head)
	if head.MatchID != nil {
		if conv, err := s.repo.AdminConversation(ctx, *head.MatchID); err == nil {
			d.Conversation = conv
			// Reading private messages is a sensitive action: always leave a trace.
			s.repo.Audit(ctx, actor, "report.read_conversation", "report", itoa64(id), map[string]any{"match_id": head.MatchID})
		}
	}
	return d, nil
}

func (s *AdminService) ResolveReport(ctx context.Context, actor, actorRole string, id int64, status, resolution, action string) error {
	switch status {
	case "open", "reviewing", "resolved", "dismissed":
	default:
		return apperr.BadRequest("invalid_status", "Invalid status")
	}
	if action == "suspend" || action == "ban" {
		target, err := s.repo.ReportedUser(ctx, id)
		if err != nil {
			return err
		}
		st := models.StatusSuspended
		if action == "ban" {
			st = models.StatusBanned
		}
		if err := s.UpdateUser(ctx, actor, actorRole, target, map[string]any{"status": st, "status_reason": resolution}); err != nil {
			return err
		}
	}
	if err := s.repo.ResolveReport(ctx, id, status, resolution, actor); err != nil {
		return err
	}
	s.repo.Audit(ctx, actor, "report.resolve", "report", itoa64(id), map[string]any{"status": status, "action": action})
	return nil
}

func itoa64(i int64) string {
	b, _ := json.Marshal(i)
	return string(b)
}

// ---- catalog (interests, languages, plans, locations…) ----

func (s *AdminService) CatalogSpec(resource string) (repository.CatalogSpec, error) {
	spec, ok := repository.CatalogSpecs[resource]
	if !ok {
		return spec, apperr.NotFound("unknown resource")
	}
	return spec, nil
}

func (s *AdminService) CatalogList(ctx context.Context, resource, q string, limit, offset int) (json.RawMessage, int, error) {
	spec, err := s.CatalogSpec(resource)
	if err != nil {
		return nil, 0, err
	}
	return s.repo.CatalogList(ctx, spec, q, limit, offset)
}

func (s *AdminService) CatalogCreate(ctx context.Context, actor, resource string, body map[string]any) (json.RawMessage, error) {
	spec, err := s.CatalogSpec(resource)
	if err != nil {
		return nil, err
	}
	out, err := s.repo.CatalogCreate(ctx, spec, body)
	if err == nil {
		s.catalog.Invalidate()
		s.repo.Audit(ctx, actor, "catalog.create", resource, "", body)
	}
	return out, err
}

func (s *AdminService) CatalogUpdate(ctx context.Context, actor, resource, id string, body map[string]any) (json.RawMessage, error) {
	spec, err := s.CatalogSpec(resource)
	if err != nil {
		return nil, err
	}
	out, err := s.repo.CatalogUpdate(ctx, spec, id, body)
	if err == nil {
		s.catalog.Invalidate()
		s.repo.Audit(ctx, actor, "catalog.update", resource, id, body)
	}
	return out, err
}

func (s *AdminService) CatalogDelete(ctx context.Context, actor, resource, id string) error {
	spec, err := s.CatalogSpec(resource)
	if err != nil {
		return err
	}
	if err := s.repo.CatalogDelete(ctx, spec, id); err != nil {
		return err
	}
	s.catalog.Invalidate()
	s.repo.Audit(ctx, actor, "catalog.delete", resource, id, nil)
	return nil
}

// ---- settings / billing / audit ----

func (s *AdminService) Settings(ctx context.Context) map[string]json.RawMessage {
	return s.settings.AdminAll(ctx)
}

func (s *AdminService) SetSetting(ctx context.Context, actor, key string, raw json.RawMessage) error {
	if err := s.settings.Set(ctx, key, raw); err != nil {
		return err
	}
	s.repo.Audit(ctx, actor, "setting.update", "setting", key, json.RawMessage(raw))
	return nil
}

func (s *AdminService) Payments(ctx context.Context, status string, limit, offset int) (json.RawMessage, error) {
	return s.repo.AdminPayments(ctx, status, limit, offset)
}

func (s *AdminService) Subscriptions(ctx context.Context, status string, limit, offset int) (json.RawMessage, error) {
	return s.repo.AdminSubscriptions(ctx, status, limit, offset)
}

func (s *AdminService) GrantPremium(ctx context.Context, actor string, id uuid.UUID, plan string, days int) error {
	if err := s.billing.Grant(ctx, id, plan, days); err != nil {
		return err
	}
	s.repo.Audit(ctx, actor, "premium.grant", "user", id.String(), map[string]any{"plan": plan, "days": days})
	return nil
}

func (s *AdminService) RevokePremium(ctx context.Context, actor string, id uuid.UUID) error {
	if err := s.billing.Revoke(ctx, id); err != nil {
		return err
	}
	s.repo.Audit(ctx, actor, "premium.revoke", "user", id.String(), nil)
	return nil
}

func (s *AdminService) AuditLog(ctx context.Context, limit, offset int) (json.RawMessage, error) {
	return s.repo.AuditLog(ctx, limit, offset)
}

// Broadcast sends an announcement to every active user through the bot, throttled to Telegram's limits.
func (s *AdminService) Broadcast(ctx context.Context, actor, text string) (int, error) {
	text, ok := util.CleanText(text, 1000)
	if !ok || text == "" {
		return 0, apperr.BadRequest("invalid_text", "Message must be 1–1000 characters")
	}
	if !s.bot.Enabled() {
		return 0, apperr.BadRequest("bot_disabled", "Telegram bot is not configured")
	}
	targets, err := s.repo.BroadcastTargets(ctx)
	if err != nil {
		return 0, err
	}
	s.repo.Audit(ctx, actor, "broadcast", "all", "", map[string]any{"recipients": len(targets), "text": text})
	go func() {
		bg := context.Background()
		tick := time.NewTicker(40 * time.Millisecond) // ~25 msg/s
		defer tick.Stop()
		sent := 0
		for _, tg := range targets {
			<-tick.C
			if err := s.bot.SendMessage(bg, tg, escapeHTML(text), nil); err == nil {
				sent++
			}
		}
		log.Printf("broadcast finished: %d/%d delivered", sent, len(targets))
	}()
	return len(targets), nil
}
