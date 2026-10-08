package middleware

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"atish/internal/apperr"
	"atish/internal/handlers/httpx"
	"atish/internal/models"
	"atish/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	CtxUserID = "uid"
	CtxRole   = "role"
	CtxActor  = "actor"
)

func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("panic: %v", r)
				httpx.Fail(c, fmt.Errorf("panic"))
			}
		}()
		c.Next()
	}
}

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		if c.Request.URL.Path == "/healthz" {
			return
		}
		log.Printf("%s %s %d %s", c.Request.Method, c.Request.URL.Path, c.Writer.Status(), time.Since(start).Round(time.Millisecond))
	}
}

func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "SAMEORIGIN")
		c.Next()
	}
}

// CORS allows the configured origins (or all, when "*"). Auth uses bearer
// tokens rather than cookies, so wildcard origins don't enable CSRF.
func CORS(origins []string) gin.HandlerFunc {
	all := len(origins) == 1 && origins[0] == "*"
	allowed := map[string]bool{}
	for _, o := range origins {
		allowed[o] = true
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && (all || allowed[origin]) {
			h := c.Writer.Header()
			if all {
				h.Set("Access-Control-Allow-Origin", "*")
			} else {
				h.Set("Access-Control-Allow-Origin", origin)
				h.Add("Vary", "Origin")
			}
			h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			h.Set("Access-Control-Max-Age", "600")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// Auth validates the bearer token, then checks the account is still active
// (cached in Redis for 30s so bans take effect almost immediately).
func Auth(auth *services.AuthService, rdb *redis.Client, touch func(uuid.UUID)) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			httpx.Fail(c, apperr.Unauthorized("Missing token"))
			return
		}
		claims, err := auth.Parse(strings.TrimPrefix(h, "Bearer "))
		if err != nil {
			httpx.Fail(c, apperr.Unauthorized("Invalid or expired token"))
			return
		}
		if claims.Subject == services.RootAdminSubject {
			c.Set(CtxRole, models.RoleAdmin)
			c.Set(CtxActor, services.RootAdminSubject)
			c.Next()
			return
		}
		uid, err := uuid.Parse(claims.Subject)
		if err != nil {
			httpx.Fail(c, apperr.Unauthorized("Invalid token"))
			return
		}
		status, role, err := auth.UserState(c.Request.Context(), uid)
		if err != nil || status == models.StatusDeleted {
			httpx.Fail(c, apperr.Unauthorized("Account not found"))
			return
		}
		if status != models.StatusActive {
			httpx.Fail(c, apperr.Forbidden("account_"+status, "Your account is "+status))
			return
		}
		c.Set(CtxUserID, uid)
		c.Set(CtxRole, role) // role from DB, not from the token
		c.Set(CtxActor, uid.String())
		// Throttled "last active" update (once per 5 minutes).
		if ok, _ := rdb.SetNX(c.Request.Context(), "touch:"+uid.String(), 1, 5*time.Minute).Result(); ok {
			go touch(uid)
		}
		c.Next()
	}
}

// RequireRole allows the listed roles. Root admin (password login) counts as admin.
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := c.GetString(CtxRole)
		for _, r := range roles {
			if r == role {
				c.Next()
				return
			}
		}
		httpx.Fail(c, apperr.Forbidden("forbidden", "You don't have access to this"))
	}
}

// RateLimit is a fixed-window limiter in Redis, keyed per user (or IP when anonymous).
// Redis failures fail open so a cache outage never takes the API down.
func RateLimit(rdb *redis.Client, name string, limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.ClientIP()
		if v, ok := c.Get(CtxUserID); ok {
			id = v.(uuid.UUID).String()
		}
		bucket := time.Now().Unix() / int64(window.Seconds())
		key := fmt.Sprintf("rl:%s:%s:%d", name, id, bucket)
		ctx := c.Request.Context()
		n, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			c.Next()
			return
		}
		if n == 1 {
			rdb.Expire(ctx, key, window+time.Second)
		}
		if int(n) > limit {
			c.Header("Retry-After", fmt.Sprint(int(window.Seconds())))
			httpx.Fail(c, apperr.TooMany("Too many requests, please slow down"))
			return
		}
		c.Next()
	}
}

func UserID(c *gin.Context) uuid.UUID {
	v, _ := c.Get(CtxUserID)
	id, _ := v.(uuid.UUID)
	return id
}
