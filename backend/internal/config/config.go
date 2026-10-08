package config

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env  string
	Port string

	DatabaseURL string
	RedisURL    string

	JWTSecret string
	JWTTTL    time.Duration

	// Telegram
	TelegramBotToken    string
	TelegramBotUsername string
	MiniAppURL          string // public URL of the frontend (also used for payment return URLs)
	EnableBotPolling    bool
	InitDataMaxAge      time.Duration

	// Auth helpers
	DevAuth          bool // allows POST /api/auth/dev (never enable in production)
	AdminUsername    string
	AdminPassword    string
	AdminTelegramIDs map[int64]bool

	// HTTP
	CORSOrigins []string
	PublicURL   string // public URL of the API (used to build signed media URLs)

	// Storage / crypto
	MediaDir      string
	EncryptionKey []byte // 32 bytes, AES-256-GCM for phone numbers
	MediaKey      []byte // HMAC key for signed media URLs

	// Payments
	StripeSecretKey     string
	StripeWebhookSecret string
}

func Load() (*Config, error) {
	c := &Config{
		Env:                 get("APP_ENV", "development"),
		Port:                get("PORT", "8080"),
		DatabaseURL:         get("DATABASE_URL", "postgres://atish:atish@localhost:5432/atish?sslmode=disable"),
		RedisURL:            get("REDIS_URL", "redis://localhost:6379/0"),
		JWTSecret:           get("JWT_SECRET", ""),
		JWTTTL:              time.Duration(getInt("JWT_TTL_HOURS", 24)) * time.Hour,
		TelegramBotToken:    get("TELEGRAM_BOT_TOKEN", ""),
		TelegramBotUsername: strings.TrimPrefix(get("TELEGRAM_BOT_USERNAME", ""), "@"),
		MiniAppURL:          strings.TrimRight(get("MINIAPP_URL", "http://localhost:5173"), "/"),
		EnableBotPolling:    getBool("TELEGRAM_BOT_POLLING", true),
		InitDataMaxAge:      time.Duration(getInt("INITDATA_MAX_AGE_SECONDS", 86400)) * time.Second,
		DevAuth:             getBool("DEV_AUTH", false),
		AdminUsername:       get("ADMIN_USERNAME", "admin"),
		AdminPassword:       get("ADMIN_PASSWORD", ""),
		AdminTelegramIDs:    map[int64]bool{},
		PublicURL:           strings.TrimRight(get("PUBLIC_URL", "http://localhost:8080"), "/"),
		MediaDir:            get("MEDIA_DIR", "./data/media"),
		StripeSecretKey:     get("STRIPE_SECRET_KEY", ""),
		StripeWebhookSecret: get("STRIPE_WEBHOOK_SECRET", ""),
	}
	for _, o := range strings.Split(get("CORS_ORIGINS", "*"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			c.CORSOrigins = append(c.CORSOrigins, o)
		}
	}
	for _, s := range strings.Split(get("ADMIN_TELEGRAM_IDS", ""), ",") {
		if id, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64); err == nil {
			c.AdminTelegramIDs[id] = true
		}
	}

	prod := c.IsProduction()
	if c.JWTSecret == "" {
		if prod {
			return nil, fmt.Errorf("JWT_SECRET is required in production")
		}
		c.JWTSecret = "dev-only-insecure-secret-change-me-please-0123456789"
		log.Println("WARN: JWT_SECRET not set, using an insecure development secret")
	}
	if prod && len(c.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}
	if prod && c.DevAuth {
		return nil, fmt.Errorf("DEV_AUTH must be disabled in production")
	}
	if prod && c.TelegramBotToken == "" {
		log.Println("WARN: TELEGRAM_BOT_TOKEN is not set; Telegram login will not work")
	}
	if prod && c.AdminPassword != "" && len(c.AdminPassword) < 12 {
		return nil, fmt.Errorf("ADMIN_PASSWORD must be at least 12 characters in production")
	}

	if k := get("ENCRYPTION_KEY", ""); k != "" {
		b, err := base64.StdEncoding.DecodeString(k)
		if err != nil || len(b) != 32 {
			return nil, fmt.Errorf("ENCRYPTION_KEY must be 32 bytes, base64 encoded (openssl rand -base64 32)")
		}
		c.EncryptionKey = b
	} else {
		if prod {
			return nil, fmt.Errorf("ENCRYPTION_KEY is required in production")
		}
		h := sha256.Sum256([]byte("enc:" + c.JWTSecret))
		c.EncryptionKey = h[:]
	}
	mk := sha256.Sum256([]byte("media:" + c.JWTSecret))
	c.MediaKey = mk[:]
	return c, nil
}

func (c *Config) IsProduction() bool { return c.Env == "production" }

func get(k, def string) string {
	if v, ok := os.LookupEnv(k); ok && v != "" {
		return v
	}
	return def
}

func getInt(k string, def int) int {
	if v, err := strconv.Atoi(get(k, "")); err == nil {
		return v
	}
	return def
}

func getBool(k string, def bool) bool {
	switch strings.ToLower(get(k, "")) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return def
}
