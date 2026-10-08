// Package services holds all business logic. HTTP handlers only translate
// requests into service calls, so web/Android/iOS clients share one engine.
package services

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"atish/internal/apperr"
	"atish/internal/models"
	"atish/internal/repository"
)

type Settings struct {
	repo *repository.Repo
	mu   sync.RWMutex
	data map[string]json.RawMessage
	at   time.Time
}

func NewSettings(repo *repository.Repo) *Settings { return &Settings{repo: repo} }

const settingsTTL = 10 * time.Second

func (s *Settings) all(ctx context.Context) map[string]json.RawMessage {
	s.mu.RLock()
	if s.data != nil && time.Since(s.at) < settingsTTL {
		d := s.data
		s.mu.RUnlock()
		return d
	}
	s.mu.RUnlock()
	d, err := s.repo.AllSettings(ctx)
	if err != nil {
		s.mu.RLock()
		defer s.mu.RUnlock()
		return s.data
	}
	s.mu.Lock()
	s.data, s.at = d, time.Now()
	s.mu.Unlock()
	return d
}

func (s *Settings) Int(ctx context.Context, key string, def int) int {
	var v int
	if raw, ok := s.all(ctx)[key]; ok && json.Unmarshal(raw, &v) == nil {
		return v
	}
	return def
}

func (s *Settings) Bool(ctx context.Context, key string, def bool) bool {
	var v bool
	if raw, ok := s.all(ctx)[key]; ok && json.Unmarshal(raw, &v) == nil {
		return v
	}
	return def
}

func (s *Settings) String(ctx context.Context, key, def string) string {
	var v string
	if raw, ok := s.all(ctx)[key]; ok && json.Unmarshal(raw, &v) == nil {
		return v
	}
	return def
}

func (s *Settings) Strings(ctx context.Context, key string) []string {
	var v []string
	if raw, ok := s.all(ctx)[key]; ok {
		_ = json.Unmarshal(raw, &v)
	}
	return v
}

func (s *Settings) Weights(ctx context.Context) Weights {
	w := DefaultWeights()
	if raw, ok := s.all(ctx)["score_weights"]; ok {
		_ = json.Unmarshal(raw, &w)
	}
	return w
}

// Public is the subset of settings that clients may read.
func (s *Settings) Public(ctx context.Context) map[string]any {
	return map[string]any{
		"min_age":          s.Int(ctx, "min_age", 18),
		"max_photos":       s.Int(ctx, "max_photos", 3),
		"free_daily_likes": s.Int(ctx, "free_daily_likes", 60),
		"premium_enabled":  s.Bool(ctx, "premium_enabled", true),
		"maintenance_mode": s.Bool(ctx, "maintenance_mode", false),
		"banner":           s.String(ctx, "banner", ""),
		"support_username": s.String(ctx, "support_username", ""),
	}
}

func (s *Settings) AdminAll(ctx context.Context) map[string]json.RawMessage {
	d, err := s.repo.AllSettings(ctx)
	if err != nil {
		return map[string]json.RawMessage{}
	}
	return d
}

var knownSettings = map[string]string{
	"min_age": "int", "max_photos": "int", "free_daily_likes": "int", "premium_enabled": "bool", "registration_open": "bool",
	"maintenance_mode": "bool", "pass_resurface_days": "int", "discovery_batch_size": "int", "banner": "string",
	"support_username": "string", "blocked_words": "strings", "score_weights": "object",
}

func (s *Settings) Set(ctx context.Context, key string, raw json.RawMessage) error {
	typ, ok := knownSettings[key]
	if !ok {
		return apperr.BadRequest("unknown_setting", "unknown setting: "+key)
	}
	var err error
	switch typ {
	case "int":
		var v int
		if err = json.Unmarshal(raw, &v); err == nil && (v < 0 || v > 100000) {
			err = apperr.BadRequest("invalid_value", "out of range")
		}
		if err == nil && key == "min_age" && (v < 13 || v > 99) {
			err = apperr.BadRequest("invalid_value", "min_age must be between 13 and 99")
		}
		if err == nil && key == "max_photos" && (v < 1 || v > 9) {
			err = apperr.BadRequest("invalid_value", "max_photos must be between 1 and 9")
		}
	case "bool":
		var v bool
		err = json.Unmarshal(raw, &v)
	case "string":
		var v string
		if err = json.Unmarshal(raw, &v); err == nil && len(v) > 500 {
			err = apperr.BadRequest("invalid_value", "too long")
		}
	case "strings":
		var v []string
		err = json.Unmarshal(raw, &v)
	case "object":
		var v map[string]float64
		err = json.Unmarshal(raw, &v)
	}
	if err != nil {
		if _, ok := apperr.As(err); ok {
			return err
		}
		return apperr.BadRequest("invalid_value", "invalid value for "+key)
	}
	if err := s.repo.SetSetting(ctx, key, raw); err != nil {
		return err
	}
	s.mu.Lock()
	s.data = nil
	s.mu.Unlock()
	return nil
}

// ---- reference catalog (cached) ----

type CatalogService struct {
	repo *repository.Repo
	mu   sync.RWMutex
	c    *models.Catalog
	at   time.Time
}

func NewCatalog(repo *repository.Repo) *CatalogService { return &CatalogService{repo: repo} }

func (c *CatalogService) Get(ctx context.Context) (*models.Catalog, error) {
	c.mu.RLock()
	if c.c != nil && time.Since(c.at) < 30*time.Second {
		defer c.mu.RUnlock()
		return c.c, nil
	}
	c.mu.RUnlock()
	cat, err := c.repo.LoadCatalog(ctx)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.c, c.at = cat, time.Now()
	c.mu.Unlock()
	return cat, nil
}

func (c *CatalogService) Invalidate() {
	c.mu.Lock()
	c.c = nil
	c.mu.Unlock()
}

// containsWord reports whether text contains a blocked word as a whole word.
func containsWord(text string, blocked []string) bool {
	fields := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r > 127)
	})
	set := map[string]bool{}
	for _, b := range blocked {
		set[strings.ToLower(b)] = true
	}
	for _, f := range fields {
		if set[f] {
			return true
		}
	}
	return false
}

// containsSubstring is the stricter check used for usernames.
func containsSubstring(text string, blocked []string) bool {
	t := strings.ToLower(strings.ReplaceAll(text, "_", ""))
	for _, b := range blocked {
		if b != "" && strings.Contains(t, strings.ToLower(b)) {
			return true
		}
	}
	return false
}
