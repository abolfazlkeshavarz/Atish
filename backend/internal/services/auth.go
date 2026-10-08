package services

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"regexp"
	"strings"
	"time"

	"atish/internal/apperr"
	"atish/internal/config"
	"atish/internal/models"
	"atish/internal/repository"
	"atish/internal/telegram"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const UsernamePrefix = "Atish_"
const RootAdminSubject = "root"

var (
	suffixRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_]{2,23}$`)
	reserved = map[string]bool{"admin": true, "administrator": true, "support": true, "atish": true, "moderator": true, "mod": true,
		"official": true, "staff": true, "team": true, "telegram": true, "system": true, "root": true, "help": true, "security": true, "bot": true}
)

type Claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

type AuthService struct {
	cfg      *config.Config
	repo     *repository.Repo
	rdb      *redis.Client
	settings *Settings
}

func NewAuth(cfg *config.Config, repo *repository.Repo, rdb *redis.Client, settings *Settings) *AuthService {
	return &AuthService{cfg: cfg, repo: repo, rdb: rdb, settings: settings}
}

type LoginResult struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	IsNew     bool      `json:"is_new"`
}

func (a *AuthService) Issue(sub, role string) (string, time.Time, error) {
	exp := time.Now().Add(a.cfg.JWTTTL)
	claims := Claims{Role: role, RegisteredClaims: jwt.RegisteredClaims{
		Subject: sub, Issuer: "atish", IssuedAt: jwt.NewNumericDate(time.Now()), ExpiresAt: jwt.NewNumericDate(exp),
	}}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(a.cfg.JWTSecret))
	return tok, exp, err
}

func (a *AuthService) Parse(token string) (*Claims, error) {
	c := &Claims{}
	t, err := jwt.ParseWithClaims(token, c, func(t *jwt.Token) (any, error) { return []byte(a.cfg.JWTSecret), nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer("atish"), jwt.WithExpirationRequired())
	if err != nil || !t.Valid {
		return nil, errors.New("invalid token")
	}
	return c, nil
}

// LoginTelegram authenticates a Telegram Mini App session. The Telegram user
// identity comes exclusively from the HMAC-validated init data.
func (a *AuthService) LoginTelegram(ctx context.Context, initData string) (*LoginResult, error) {
	tu, err := telegram.ValidateInitData(initData, a.cfg.TelegramBotToken, a.cfg.InitDataMaxAge, time.Now())
	if err != nil {
		return nil, apperr.Unauthorized("Telegram authentication failed")
	}
	return a.loginTelegramUser(ctx, tu)
}

// LoginDev signs in as a synthetic Telegram user. Only mounted when DEV_AUTH=true.
func (a *AuthService) LoginDev(ctx context.Context, tgID int64, name string) (*LoginResult, error) {
	if name == "" {
		name = fmt.Sprintf("dev%d", tgID)
	}
	return a.loginTelegramUser(ctx, &telegram.User{ID: tgID, FirstName: name, Username: strings.ToLower(strings.ReplaceAll(name, " ", "_"))})
}

func (a *AuthService) loginTelegramUser(ctx context.Context, tu *telegram.User) (*LoginResult, error) {
	role := models.RoleUser
	if a.cfg.AdminTelegramIDs[tu.ID] {
		role = models.RoleAdmin
	}
	user, err := a.repo.GetUserByTelegramID(ctx, tu.ID)
	isNew := false
	switch {
	case err == nil:
		if err := a.repo.UpdateTelegramInfo(ctx, user.ID, tu.Username, tu.PhotoURL, tu.LanguageCode); err != nil {
			return nil, err
		}
		if role == models.RoleAdmin && user.Role != models.RoleAdmin {
			if err := a.repo.AdminSetUser(ctx, user.ID, map[string]any{"role": models.RoleAdmin}); err != nil {
				return nil, err
			}
			user.Role = models.RoleAdmin
		}
	case errors.Is(err, apperr.ErrNotFound):
		if role != models.RoleAdmin && !a.settings.Bool(ctx, "registration_open", true) {
			return nil, apperr.Forbidden("registration_closed", "Registration is temporarily closed")
		}
		isNew = true
		for attempt := 0; ; attempt++ {
			var name *string
			if g, gerr := a.GenerateUsername(ctx, tu.Username, attempt); gerr == nil {
				name = &g
			}
			user, err = a.repo.CreateTelegramUser(ctx, tu.ID, tu.Username, tu.PhotoURL, tu.LanguageCode, name, role)
			if err == nil {
				break
			}
			if repository.IsUnique(err) {
				// Either a username race (retry with a different variant) or a double sign-up.
				if existing, e2 := a.repo.GetUserByTelegramID(ctx, tu.ID); e2 == nil {
					user, isNew = existing, false
					break
				}
				if attempt < 5 {
					continue
				}
			}
			return nil, err
		}
	default:
		return nil, err
	}

	switch user.Status {
	case models.StatusBanned:
		return nil, apperr.Forbidden("account_banned", "This account has been banned").With("reason", user.StatusReason)
	case models.StatusSuspended:
		return nil, apperr.Forbidden("account_suspended", "This account is suspended").With("reason", user.StatusReason)
	case models.StatusDeleted:
		return nil, apperr.Unauthorized("Account deleted")
	}
	tok, exp, err := a.Issue(user.ID.String(), user.Role)
	if err != nil {
		return nil, err
	}
	return &LoginResult{Token: tok, ExpiresAt: exp, IsNew: isNew}, nil
}

// AdminLogin is the credential login for the browser admin panel.
func (a *AuthService) AdminLogin(username, password string) (*LoginResult, error) {
	if a.cfg.AdminPassword == "" {
		return nil, apperr.Forbidden("admin_login_disabled", "Admin password login is not configured")
	}
	u := subtle.ConstantTimeCompare([]byte(username), []byte(a.cfg.AdminUsername))
	p := subtle.ConstantTimeCompare([]byte(password), []byte(a.cfg.AdminPassword))
	if u&p != 1 {
		return nil, apperr.Unauthorized("Invalid credentials")
	}
	tok, exp, err := a.Issue(RootAdminSubject, models.RoleAdmin)
	if err != nil {
		return nil, err
	}
	return &LoginResult{Token: tok, ExpiresAt: exp}, nil
}

// ---- cached user state (so banned users are cut off within seconds) ----

type userState struct {
	Status string `json:"s"`
	Role   string `json:"r"`
}

func (a *AuthService) UserState(ctx context.Context, id uuid.UUID) (status, role string, err error) {
	key := "ustate:" + id.String()
	if raw, e := a.rdb.Get(ctx, key).Bytes(); e == nil {
		var st userState
		if json.Unmarshal(raw, &st) == nil {
			return st.Status, st.Role, nil
		}
	}
	u, err := a.repo.GetUser(ctx, id)
	if err != nil {
		return "", "", err
	}
	b, _ := json.Marshal(userState{u.Status, u.Role})
	a.rdb.Set(ctx, key, b, 30*time.Second)
	return u.Status, u.Role, nil
}

func (a *AuthService) InvalidateUser(ctx context.Context, id uuid.UUID) {
	a.rdb.Del(ctx, "ustate:"+id.String())
}

// ---- usernames ----

func (a *AuthService) ValidateSuffix(ctx context.Context, suffix string) error {
	if !suffixRe.MatchString(suffix) {
		return apperr.BadRequest("invalid_username", "Use 3–24 letters, numbers or underscores")
	}
	if strings.Contains(suffix, "__") || strings.HasSuffix(suffix, "_") {
		return apperr.BadRequest("invalid_username", "No double or trailing underscores")
	}
	if reserved[strings.ToLower(suffix)] {
		return apperr.BadRequest("username_reserved", "That username is reserved")
	}
	if containsSubstring(suffix, a.settings.Strings(ctx, "blocked_words")) {
		return apperr.BadRequest("username_not_allowed", "That username is not allowed")
	}
	return nil
}

// CheckUsername validates and reports availability. current is the caller's own name (always "available").
func (a *AuthService) CheckUsername(ctx context.Context, suffix string, current *string) error {
	if err := a.ValidateSuffix(ctx, suffix); err != nil {
		return err
	}
	full := UsernamePrefix + suffix
	if current != nil && strings.EqualFold(*current, full) {
		return nil
	}
	taken, err := a.repo.UsernameTaken(ctx, full)
	if err != nil {
		return err
	}
	if taken {
		return apperr.Conflict("username_taken", "That username is already taken")
	}
	return nil
}

var nonName = regexp.MustCompile(`[^A-Za-z0-9_]`)

// GenerateUsername derives Atish_<telegram_username> and, when taken, an
// available variant (…2, …24, …381). attempt>0 forces a variant after a race.
func (a *AuthService) GenerateUsername(ctx context.Context, tgUsername string, attempt int) (string, error) {
	base := nonName.ReplaceAllString(tgUsername, "")
	base = strings.Trim(base, "_")
	for strings.Contains(base, "__") {
		base = strings.ReplaceAll(base, "__", "_")
	}
	if len(base) > 20 {
		base = base[:20]
	}
	if len(base) < 3 || a.ValidateSuffix(ctx, base) != nil {
		return "", errors.New("no usable telegram username")
	}
	cands := []string{base}
	for i := 2; i <= 9; i++ {
		cands = append(cands, fmt.Sprintf("%s%d", base, i))
	}
	for i := 0; i < 8; i++ {
		cands = append(cands, fmt.Sprintf("%s%d", base, 10+rand.IntN(90)))
	}
	for i := 0; i < 8; i++ {
		cands = append(cands, fmt.Sprintf("%s%d", base, 100+rand.IntN(900)))
	}
	if attempt > 0 {
		cands = cands[1:]
	}
	for _, c := range cands {
		taken, err := a.repo.UsernameTaken(ctx, UsernamePrefix+c)
		if err != nil {
			return "", err
		}
		if !taken {
			return UsernamePrefix + c, nil
		}
	}
	return "", errors.New("could not generate username")
}
