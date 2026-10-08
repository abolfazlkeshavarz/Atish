package services

import (
	"context"
	"strings"

	"atish/internal/apperr"
	"atish/internal/models"
	"atish/internal/repository"
	"atish/internal/util"

	"github.com/google/uuid"
)

type ChatService struct {
	repo      *repository.Repo
	settings  *Settings
	discovery *DiscoveryService
	notifier  *Notifier
}

func NewChat(repo *repository.Repo, settings *Settings, discovery *DiscoveryService, notifier *Notifier) *ChatService {
	return &ChatService{repo: repo, settings: settings, discovery: discovery, notifier: notifier}
}

func (s *ChatService) List(ctx context.Context, me uuid.UUID) ([]models.Chat, error) {
	rows, err := s.repo.ListChats(ctx, me)
	if err != nil {
		return nil, err
	}
	mrows := make([]repository.MatchRow, len(rows))
	for i, r := range rows {
		mrows[i] = r.Match
	}
	matches, err := s.discovery.hydrate(ctx, me, mrows)
	if err != nil {
		return nil, err
	}
	byID := map[uuid.UUID]*models.Match{}
	for i := range matches {
		byID[matches[i].ID] = &matches[i]
	}
	out := make([]models.Chat, 0, len(rows))
	for _, r := range rows {
		if m := byID[r.Match.ID]; m != nil {
			out = append(out, models.Chat{Match: m, LastMessage: r.LastMessage, Unread: r.Unread})
		}
	}
	return out, nil
}

// membership returns the match if me is a participant, the match is active and neither side is blocked.
func (s *ChatService) membership(ctx context.Context, me, matchID uuid.UUID) (*repository.MatchRow, error) {
	m, err := s.repo.GetMatch(ctx, matchID)
	if err != nil || !m.Has(me) || m.UnmatchedAt != nil {
		return nil, apperr.NotFound("chat not found")
	}
	if blocked, err := s.repo.IsBlockedEither(ctx, me, m.Other(me)); err != nil || blocked {
		return nil, apperr.NotFound("chat not found")
	}
	return m, nil
}

func (s *ChatService) Messages(ctx context.Context, me, matchID uuid.UUID, afterID, beforeID int64, limit int) ([]models.Message, error) {
	if _, err := s.membership(ctx, me, matchID); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.repo.ListMessages(ctx, matchID, afterID, beforeID, limit)
}

func (s *ChatService) Send(ctx context.Context, me, matchID uuid.UUID, body string) (*models.Message, error) {
	m, err := s.membership(ctx, me, matchID)
	if err != nil {
		return nil, err
	}
	body, ok := util.CleanText(body, 2000)
	if !ok || strings.TrimSpace(body) == "" {
		return nil, apperr.BadRequest("invalid_message", "Message must be 1–2000 characters")
	}
	other := m.Other(me)
	if st, _, err := s.discoveryAuthState(ctx, other); err != nil || st != models.StatusActive {
		return nil, apperr.NotFound("chat not found")
	}
	msg, err := s.repo.InsertMessage(ctx, matchID, me, body)
	if err != nil {
		return nil, err
	}
	if p, err := s.repo.LoadProfile(ctx, me); err == nil {
		s.notifier.NewMessage(ctx, other, p.DisplayName, matchID)
	}
	return msg, nil
}

func (s *ChatService) discoveryAuthState(ctx context.Context, id uuid.UUID) (string, string, error) {
	u, err := s.repo.GetUser(ctx, id)
	if err != nil {
		return "", "", err
	}
	return u.Status, u.Role, nil
}

func (s *ChatService) MarkRead(ctx context.Context, me, matchID uuid.UUID) error {
	if _, err := s.membership(ctx, me, matchID); err != nil {
		return err
	}
	return s.repo.MarkRead(ctx, matchID, me)
}

func (s *ChatService) Unread(ctx context.Context, me uuid.UUID) (int, error) {
	return s.repo.TotalUnread(ctx, me)
}
