package services

import (
	"context"

	"atish/internal/apperr"
	"atish/internal/models"
	"atish/internal/repository"
	"atish/internal/util"

	"github.com/google/uuid"
)

type SafetyService struct {
	repo      *repository.Repo
	presenter *Presenter
}

func NewSafety(repo *repository.Repo, presenter *Presenter) *SafetyService {
	return &SafetyService{repo: repo, presenter: presenter}
}

var reportReasons = []string{"fake_profile", "inappropriate_photo", "harassment", "spam_scam", "underage", "hate_speech", "other"}

func (s *SafetyService) Block(ctx context.Context, me, target uuid.UUID) error {
	if me == target {
		return apperr.BadRequest("invalid_target", "You can't block yourself")
	}
	if _, err := s.repo.GetUser(ctx, target); err != nil {
		return err
	}
	return s.repo.Block(ctx, me, target)
}

func (s *SafetyService) Unblock(ctx context.Context, me, target uuid.UUID) error {
	return s.repo.Unblock(ctx, me, target)
}

type BlockedUser struct {
	ID          uuid.UUID `json:"id"`
	DisplayName string    `json:"display_name"`
	Username    string    `json:"atish_username"`
}

func (s *SafetyService) Blocked(ctx context.Context, me uuid.UUID) ([]BlockedUser, error) {
	ids, err := s.repo.ListBlocked(ctx, me)
	if err != nil {
		return nil, err
	}
	ps, err := s.repo.LoadProfiles(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := []BlockedUser{}
	for _, id := range ids {
		if p := ps[id]; p != nil {
			b := BlockedUser{ID: id, DisplayName: p.DisplayName}
			if p.AtishUsername != nil {
				b.Username = *p.AtishUsername
			}
			out = append(out, b)
		}
	}
	return out, nil
}

func (s *SafetyService) Report(ctx context.Context, me, target uuid.UUID, reason, details string) error {
	if me == target {
		return apperr.BadRequest("invalid_target", "You can't report yourself")
	}
	if !util.Contains(reportReasons, reason) {
		return apperr.BadRequest("invalid_reason", "Choose a valid reason")
	}
	details, ok := util.CleanText(details, 1000)
	if !ok {
		return apperr.BadRequest("invalid_details", "Details can be at most 1000 characters")
	}
	if _, err := s.repo.GetUser(ctx, target); err != nil {
		return err
	}
	if n, err := s.repo.RecentReportCount(ctx, me, target); err == nil && n >= 3 {
		return apperr.TooMany("You already reported this user")
	}
	var matchID *uuid.UUID
	if m, err := s.repo.GetMatchBetween(ctx, me, target); err == nil {
		matchID = &m.ID
	}
	_, err := s.repo.CreateReport(ctx, me, target, matchID, reason, details)
	return err
}

var _ = models.StatusActive
