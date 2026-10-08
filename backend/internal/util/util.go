package util

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// AgeOn returns the full years between birth and now.
func AgeOn(birth, now time.Time) int {
	y := now.Year() - birth.Year()
	if now.Month() < birth.Month() || (now.Month() == birth.Month() && now.Day() < birth.Day()) {
		y--
	}
	return y
}

func Encrypt(key, plain []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plain, nil), nil
}

func Decrypt(key, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(data) < gcm.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}
	return gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], nil)
}

func HashHex(pepper []byte, s string) string {
	h := sha256.New()
	h.Write(pepper)
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

// CleanText trims, collapses control characters and enforces a rune limit.
func CleanText(s string, maxRunes int) (string, bool) {
	s = strings.TrimSpace(s)
	for _, r := range s {
		if r < 0x20 && r != '\n' && r != '\t' {
			return "", false
		}
	}
	if utf8.RuneCountInString(s) > maxRunes {
		return "", false
	}
	return s, true
}

func Contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func Intersect(a, b []string) []string {
	set := make(map[string]struct{}, len(a))
	for _, x := range a {
		set[x] = struct{}{}
	}
	out := []string{}
	for _, x := range b {
		if _, ok := set[x]; ok {
			out = append(out, x)
		}
	}
	return out
}

func Dedup(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, x := range in {
		if _, ok := seen[x]; !ok {
			seen[x] = struct{}{}
			out = append(out, x)
		}
	}
	return out
}
