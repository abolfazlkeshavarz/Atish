package telegram

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

func sign(token string, vals url.Values) string {
	pairs := []string{}
	for k, v := range vals {
		pairs = append(pairs, k+"="+v[0])
	}
	sort.Strings(pairs)
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte(token))
	mac := hmac.New(sha256.New, secret.Sum(nil))
	mac.Write([]byte(strings.Join(pairs, "\n")))
	return hex.EncodeToString(mac.Sum(nil))
}

func build(token string, authDate time.Time, tamper bool) string {
	v := url.Values{}
	v.Set("auth_date", strconv.FormatInt(authDate.Unix(), 10))
	v.Set("query_id", "AAH")
	v.Set("user", `{"id":123456789,"first_name":"Alex","username":"alex_rossi"}`)
	v.Set("hash", sign(token, v))
	if tamper {
		v.Set("user", `{"id":42,"first_name":"Evil"}`)
	}
	return v.Encode()
}

func TestValidateInitData(t *testing.T) {
	const token = "123:ABC"
	now := time.Now()
	u, err := ValidateInitData(build(token, now, false), token, time.Hour, now)
	if err != nil || u.ID != 123456789 || u.Username != "alex_rossi" {
		t.Fatalf("valid init data rejected: %v %+v", err, u)
	}
	if _, err := ValidateInitData(build(token, now, true), token, time.Hour, now); err == nil {
		t.Fatal("tampered user id must be rejected")
	}
	if _, err := ValidateInitData(build("other:TOKEN", now, false), token, time.Hour, now); err == nil {
		t.Fatal("data signed with another bot token must be rejected")
	}
	if _, err := ValidateInitData(build(token, now.Add(-48*time.Hour), false), token, time.Hour, now); err != ErrExpiredInitData {
		t.Fatalf("expired data must be rejected, got %v", err)
	}
	if _, err := ValidateInitData("", token, time.Hour, now); err == nil {
		t.Fatal("empty init data must be rejected")
	}
}
