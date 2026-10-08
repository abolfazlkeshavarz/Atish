// Package telegram contains Telegram-specific code: Mini App init-data
// validation and a minimal Bot API client. Nothing outside this package and
// the auth/bot services knows about Telegram, so other clients can be added.
package telegram

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type User struct {
	ID           int64  `json:"id"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Username     string `json:"username"`
	LanguageCode string `json:"language_code"`
	PhotoURL     string `json:"photo_url"`
	IsPremium    bool   `json:"is_premium"`
}

var (
	ErrInvalidInitData = errors.New("invalid telegram init data")
	ErrExpiredInitData = errors.New("telegram init data expired")
)

// ValidateInitData verifies the HMAC signature of Telegram Mini App
// initData (https://core.telegram.org/bots/webapps#validating-data-received-via-the-mini-app)
// and returns the authenticated user. The Telegram ID supplied by the client
// is never trusted on its own; only data covered by the signature is used.
func ValidateInitData(initData, botToken string, maxAge time.Duration, now time.Time) (*User, error) {
	if initData == "" || botToken == "" {
		return nil, ErrInvalidInitData
	}
	vals, err := url.ParseQuery(initData)
	if err != nil {
		return nil, ErrInvalidInitData
	}
	hash := vals.Get("hash")
	if hash == "" {
		return nil, ErrInvalidInitData
	}
	pairs := make([]string, 0, len(vals))
	for k, v := range vals {
		if k == "hash" || len(v) == 0 {
			continue
		}
		pairs = append(pairs, k+"="+v[0])
	}
	sort.Strings(pairs)
	dataCheck := strings.Join(pairs, "\n")

	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte(botToken))
	mac := hmac.New(sha256.New, secret.Sum(nil))
	mac.Write([]byte(dataCheck))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(strings.ToLower(hash))) {
		return nil, ErrInvalidInitData
	}

	authDate, err := strconv.ParseInt(vals.Get("auth_date"), 10, 64)
	if err != nil {
		return nil, ErrInvalidInitData
	}
	if maxAge > 0 && now.Sub(time.Unix(authDate, 0)) > maxAge {
		return nil, ErrExpiredInitData
	}

	var u User
	if err := json.Unmarshal([]byte(vals.Get("user")), &u); err != nil || u.ID == 0 {
		return nil, ErrInvalidInitData
	}
	return &u, nil
}
