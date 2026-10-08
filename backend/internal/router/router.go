package router

import (
	"context"
	"net/http"
	"time"

	"atish/internal/apperr"
	"atish/internal/config"
	"atish/internal/handlers"
	"atish/internal/handlers/httpx"
	"atish/internal/middleware"
	"atish/internal/models"
	"atish/internal/repository"
	"atish/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func New(cfg *config.Config, h *handlers.Handlers, repo *repository.Repo, rdb *redis.Client) *gin.Engine {
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	_ = r.SetTrustedProxies([]string{"127.0.0.1", "::1", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"}) // reverse proxies (nginx/Caddy) in Docker
	r.Use(middleware.Recovery(), middleware.Logger(), middleware.SecurityHeaders(), middleware.CORS(cfg.CORSOrigins))

	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	r.GET("/media/:id", h.ServeMedia)

	api := r.Group("/api")
	api.GET("/catalog", h.GetCatalog)
	api.POST("/auth/telegram", middleware.RateLimit(rdb, "auth", 30, time.Minute), h.LoginTelegram)
	if cfg.DevAuth {
		api.POST("/auth/dev", h.LoginDev)
	}
	api.POST("/admin/login", middleware.RateLimit(rdb, "adminlogin", 8, time.Minute), h.AdminLogin)
	api.POST("/billing/webhook/stripe", h.StripeWebhook)

	touch := func(id uuid.UUID) { repo.Touch(backgroundCtx(), id) }
	authed := api.Group("", middleware.Auth(h.Auth, rdb, touch), middleware.RateLimit(rdb, "api", 240, time.Minute), maintenance(h.Settings))

	// Account
	authed.GET("/me", h.Me)
	authed.PATCH("/me/profile", middleware.RateLimit(rdb, "profile", 60, time.Minute), h.UpdateProfile)
	authed.GET("/me/preferences", h.GetPreferences)
	authed.PATCH("/me/preferences", middleware.RateLimit(rdb, "prefs", 60, time.Minute), h.UpdatePreferences)
	authed.GET("/username/check", middleware.RateLimit(rdb, "ucheck", 60, time.Minute), h.CheckUsername)
	authed.PUT("/me/username", middleware.RateLimit(rdb, "uset", 10, time.Minute), h.SetUsername)
	authed.POST("/me/onboarding/complete", h.CompleteOnboarding)
	authed.POST("/me/photos", middleware.RateLimit(rdb, "photo", 20, time.Hour), h.UploadPhoto)
	authed.POST("/me/photos/telegram", middleware.RateLimit(rdb, "photo", 20, time.Hour), h.ImportTelegramPhoto)
	authed.PUT("/me/photos/order", h.ReorderPhotos)
	authed.DELETE("/me/photos/:id", h.DeletePhoto)
	authed.DELETE("/me/phone", h.RemovePhone)
	authed.GET("/me/blocks", h.Blocked)
	authed.DELETE("/me", middleware.RateLimit(rdb, "delete", 5, time.Hour), h.DeleteAccount)

	authed.GET("/locations/cities", h.SearchCities)
	authed.GET("/locations/areas", h.ListAreas)
	authed.POST("/locations", middleware.RateLimit(rdb, "loc", 10, time.Hour), h.CreateLocation)

	// Discovery & matching
	authed.GET("/discover", h.Discover)
	authed.POST("/discover/rewind", h.Rewind)
	authed.POST("/discover/:id/like", middleware.RateLimit(rdb, "like", 120, time.Minute), h.Like)
	authed.POST("/discover/:id/pass", middleware.RateLimit(rdb, "pass", 240, time.Minute), h.Pass)
	authed.GET("/likes/received", h.LikesReceived)
	authed.GET("/matches", h.Matches)
	authed.DELETE("/matches/:id", h.Unmatch)
	authed.GET("/users/:id", h.MatchedProfile)

	// Chat
	authed.GET("/chats", h.Chats)
	authed.GET("/chats/unread", h.Unread)
	authed.GET("/chats/:id/messages", h.Messages)
	authed.POST("/chats/:id/messages", middleware.RateLimit(rdb, "msg", 40, time.Minute), h.SendMessage)
	authed.POST("/chats/:id/read", h.MarkRead)

	// Safety
	authed.POST("/users/:id/block", middleware.RateLimit(rdb, "block", 30, time.Hour), h.Block)
	authed.DELETE("/users/:id/block", h.Unblock)
	authed.POST("/users/:id/report", middleware.RateLimit(rdb, "report", 20, time.Hour), h.Report)

	// Billing
	authed.GET("/billing/plans", h.Plans)
	authed.GET("/billing/entitlements", h.Entitlements)
	authed.POST("/billing/checkout", middleware.RateLimit(rdb, "checkout", 10, time.Minute), h.Checkout)

	// Admin panel API: moderators get the moderation subset, admins get everything.
	mod := api.Group("/admin", middleware.Auth(h.Auth, rdb, touch), middleware.RequireRole(models.RoleAdmin, models.RoleModerator))
	mod.GET("/whoami", h.AdminWhoAmI)
	mod.GET("/stats", h.AdminStats)
	mod.GET("/users", h.AdminUsers)
	mod.GET("/users/:id", h.AdminUser)
	mod.PATCH("/users/:id", h.AdminUpdateUser)
	mod.PATCH("/users/:id/profile", h.AdminEditProfile)
	mod.GET("/photos", h.AdminRecentPhotos)
	mod.DELETE("/photos/:id", h.AdminRemovePhoto)
	mod.GET("/reports", h.AdminReports)
	mod.GET("/reports/:id", h.AdminReport)
	mod.PATCH("/reports/:id", h.AdminResolveReport)

	adm := mod.Group("", middleware.RequireRole(models.RoleAdmin))
	adm.DELETE("/users/:id", h.AdminDeleteUser)
	adm.POST("/users/:id/premium", h.AdminGrantPremium)
	adm.DELETE("/users/:id/premium", h.AdminRevokePremium)
	adm.GET("/catalog/:resource", h.AdminCatalogList)
	adm.POST("/catalog/:resource", h.AdminCatalogCreate)
	adm.PATCH("/catalog/:resource/:id", h.AdminCatalogUpdate)
	adm.DELETE("/catalog/:resource/:id", h.AdminCatalogDelete)
	adm.GET("/settings", h.AdminSettings)
	adm.PUT("/settings/:key", h.AdminSetSetting)
	adm.GET("/payments", h.AdminPayments)
	adm.GET("/subscriptions", h.AdminSubscriptions)
	adm.GET("/audit", h.AdminAudit)
	adm.POST("/broadcast", middleware.RateLimit(rdb, "broadcast", 3, time.Hour), h.AdminBroadcast)

	return r
}

// maintenance returns 503 for non-staff users while maintenance mode is on.
func maintenance(s *services.Settings) gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.Bool(c.Request.Context(), "maintenance_mode", false) {
			role := c.GetString(middleware.CtxRole)
			if role != models.RoleAdmin && role != models.RoleModerator {
				httpx.Fail(c, apperr.New(http.StatusServiceUnavailable, "maintenance", "Atish is undergoing maintenance. Please come back soon."))
				return
			}
		}
		c.Next()
	}
}

func backgroundCtx() context.Context { return context.Background() }
