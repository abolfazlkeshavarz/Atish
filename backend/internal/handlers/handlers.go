// Package handlers contains thin HTTP adapters. They parse input, call a
// service and write JSON: no business rules live here.
package handlers

import (
	"io"
	"net/http"
	"strings"
	"time"

	"atish/internal/apperr"
	"atish/internal/config"
	"atish/internal/handlers/httpx"
	"atish/internal/middleware"
	"atish/internal/models"
	"atish/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handlers struct {
	Cfg       *config.Config
	Auth      *services.AuthService
	Profile   *services.ProfileService
	Discovery *services.DiscoveryService
	Chat      *services.ChatService
	Safety    *services.SafetyService
	Billing   *services.BillingService
	Admin     *services.AdminService
	Settings  *services.Settings
	Catalog   *services.CatalogService
}

// ---- auth ----

func (h *Handlers) LoginTelegram(c *gin.Context) {
	var in struct {
		InitData string `json:"init_data"`
	}
	if !httpx.Bind(c, &in) {
		return
	}
	res, err := h.Auth.LoginTelegram(c.Request.Context(), in.InitData)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, res)
}

// LoginDev is only routed when DEV_AUTH=true.
func (h *Handlers) LoginDev(c *gin.Context) {
	var in struct {
		TelegramID int64  `json:"telegram_id"`
		Name       string `json:"name"`
	}
	if !httpx.Bind(c, &in) {
		return
	}
	if in.TelegramID <= 0 {
		httpx.Fail(c, apperr.BadRequest("invalid_id", "telegram_id required"))
		return
	}
	res, err := h.Auth.LoginDev(c.Request.Context(), in.TelegramID, in.Name)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, res)
}

func (h *Handlers) AdminLogin(c *gin.Context) {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !httpx.Bind(c, &in) {
		return
	}
	res, err := h.Auth.AdminLogin(in.Username, in.Password)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, res)
}

// ---- catalog & locations ----

func (h *Handlers) GetCatalog(c *gin.Context) {
	cat, err := h.Catalog.Get(c.Request.Context())
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.Header("Cache-Control", "public, max-age=60")
	httpx.OK(c, gin.H{"catalog": cat, "app": h.Settings.Public(c.Request.Context())})
}

func (h *Handlers) SearchCities(c *gin.Context) {
	res, err := h.Profile.SearchCities(c.Request.Context(), c.Query("country"), c.Query("q"))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": res})
}

func (h *Handlers) ListAreas(c *gin.Context) {
	res, err := h.Profile.ListAreas(c.Request.Context(), c.Query("country"), c.Query("city"))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": res})
}

func (h *Handlers) CreateLocation(c *gin.Context) {
	var in models.Location
	if !httpx.Bind(c, &in) {
		return
	}
	res, err := h.Profile.CreateLocation(c.Request.Context(), in)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, res)
}

// ---- me ----

func (h *Handlers) Me(c *gin.Context) {
	me, err := h.Profile.Me(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, me)
}

func (h *Handlers) UpdateProfile(c *gin.Context) {
	var in models.ProfilePatch
	if !httpx.Bind(c, &in) {
		return
	}
	if err := h.Profile.UpdateProfile(c.Request.Context(), middleware.UserID(c), in); err != nil {
		httpx.Fail(c, err)
		return
	}
	h.Me(c)
}

func (h *Handlers) GetPreferences(c *gin.Context) {
	me, err := h.Profile.Me(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, gin.H{
		"friendship_kinds": me.FriendshipKinds, "preferences": me.Preferences, "personality": me.Personality,
		"availability": me.Availability, "privacy": me.Privacy,
	})
}

func (h *Handlers) UpdatePreferences(c *gin.Context) {
	var in models.PreferencesPatch
	if !httpx.Bind(c, &in) {
		return
	}
	if err := h.Profile.UpdatePreferences(c.Request.Context(), middleware.UserID(c), in); err != nil {
		httpx.Fail(c, err)
		return
	}
	h.Me(c)
}

func (h *Handlers) CheckUsername(c *gin.Context) {
	ctx := c.Request.Context()
	u, err := h.Profile.Me(ctx, middleware.UserID(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	if err := h.Auth.CheckUsername(ctx, c.Query("suffix"), u.AtishUsername); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, gin.H{"available": true, "username": services.UsernamePrefix + c.Query("suffix")})
}

func (h *Handlers) SetUsername(c *gin.Context) {
	var in struct {
		Suffix string `json:"suffix"`
	}
	if !httpx.Bind(c, &in) {
		return
	}
	if err := h.Profile.SetUsername(c.Request.Context(), middleware.UserID(c), strings.TrimPrefix(in.Suffix, services.UsernamePrefix)); err != nil {
		httpx.Fail(c, err)
		return
	}
	h.Me(c)
}

func (h *Handlers) CompleteOnboarding(c *gin.Context) {
	if err := h.Profile.CompleteOnboarding(c.Request.Context(), middleware.UserID(c)); err != nil {
		httpx.Fail(c, err)
		return
	}
	h.Me(c)
}

func (h *Handlers) UploadPhoto(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 9<<20)
	fh, err := c.FormFile("photo")
	if err != nil {
		httpx.Fail(c, apperr.BadRequest("invalid_image", "Attach an image in the 'photo' field"))
		return
	}
	f, err := fh.Open()
	if err != nil {
		httpx.Fail(c, apperr.BadRequest("invalid_image", "Could not read image"))
		return
	}
	defer f.Close()
	ph, err := h.Profile.AddPhoto(c.Request.Context(), middleware.UserID(c), io.Reader(f))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, ph)
}

func (h *Handlers) ImportTelegramPhoto(c *gin.Context) {
	ph, err := h.Profile.ImportTelegramPhoto(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, ph)
}

func (h *Handlers) DeletePhoto(c *gin.Context) {
	id, ok := httpx.ParamUUID(c, "id")
	if !ok {
		return
	}
	if err := h.Profile.DeletePhoto(c.Request.Context(), middleware.UserID(c), id); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}

func (h *Handlers) ReorderPhotos(c *gin.Context) {
	var in struct {
		IDs []uuid.UUID `json:"ids"`
	}
	if !httpx.Bind(c, &in) {
		return
	}
	if err := h.Profile.ReorderPhotos(c.Request.Context(), middleware.UserID(c), in.IDs); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}

func (h *Handlers) RemovePhone(c *gin.Context) {
	if err := h.Profile.RemovePhone(c.Request.Context(), middleware.UserID(c)); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}

func (h *Handlers) DeleteAccount(c *gin.Context) {
	var in struct {
		Confirm string `json:"confirm"`
	}
	if !httpx.Bind(c, &in) {
		return
	}
	if in.Confirm != "DELETE" {
		httpx.Fail(c, apperr.BadRequest("confirmation_required", "Type DELETE to confirm"))
		return
	}
	if err := h.Profile.DeleteAccount(c.Request.Context(), middleware.UserID(c)); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}

// ServeMedia streams a photo for a valid signed URL.
func (h *Handlers) ServeMedia(c *gin.Context) {
	id, ok := httpx.ParamUUID(c, "id")
	if !ok {
		return
	}
	data, err := h.Profile.ServePhoto(c.Request.Context(), id, c.Query("e"), c.Query("s"))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.Header("Cache-Control", "private, max-age=43200")
	c.Header("Cross-Origin-Resource-Policy", "cross-origin")
	c.Data(http.StatusOK, "image/jpeg", data)
}

// ---- discovery / matches ----

func (h *Handlers) Discover(c *gin.Context) {
	f := services.FeedFilter{
		Type: c.Query("type"), AgeMin: httpx.QueryInt(c, "min_age", 0), AgeMax: httpx.QueryInt(c, "max_age", 0),
		City: c.Query("city"), Language: c.Query("language"), Limit: httpx.QueryInt(c, "limit", 0),
	}
	if v := c.Query("interests"); v != "" {
		f.Interests = strings.Split(v, ",")
	}
	res, err := h.Discovery.Feed(c.Request.Context(), middleware.UserID(c), f)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, res)
}

func (h *Handlers) Like(c *gin.Context) {
	id, ok := httpx.ParamUUID(c, "id")
	if !ok {
		return
	}
	res, err := h.Discovery.Like(c.Request.Context(), middleware.UserID(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, res)
}

func (h *Handlers) Pass(c *gin.Context) {
	id, ok := httpx.ParamUUID(c, "id")
	if !ok {
		return
	}
	if err := h.Discovery.Pass(c.Request.Context(), middleware.UserID(c), id); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}

func (h *Handlers) Rewind(c *gin.Context) {
	p, err := h.Discovery.Rewind(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, p)
}

func (h *Handlers) LikesReceived(c *gin.Context) {
	res, err := h.Discovery.LikesReceived(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, res)
}

func (h *Handlers) Matches(c *gin.Context) {
	res, err := h.Discovery.Matches(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": res})
}

func (h *Handlers) Unmatch(c *gin.Context) {
	id, ok := httpx.ParamUUID(c, "id")
	if !ok {
		return
	}
	if err := h.Discovery.Unmatch(c.Request.Context(), middleware.UserID(c), id); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}

func (h *Handlers) MatchedProfile(c *gin.Context) {
	id, ok := httpx.ParamUUID(c, "id")
	if !ok {
		return
	}
	p, err := h.Discovery.MatchProfile(c.Request.Context(), middleware.UserID(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, p)
}

// ---- chat ----

func (h *Handlers) Chats(c *gin.Context) {
	res, err := h.Chat.List(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": res})
}

func (h *Handlers) Unread(c *gin.Context) {
	n, err := h.Chat.Unread(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, gin.H{"unread": n})
}

func (h *Handlers) Messages(c *gin.Context) {
	id, ok := httpx.ParamUUID(c, "id")
	if !ok {
		return
	}
	res, err := h.Chat.Messages(c.Request.Context(), middleware.UserID(c), id,
		httpx.QueryInt64(c, "after"), httpx.QueryInt64(c, "before"), httpx.QueryInt(c, "limit", 50))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": res})
}

func (h *Handlers) SendMessage(c *gin.Context) {
	id, ok := httpx.ParamUUID(c, "id")
	if !ok {
		return
	}
	var in struct {
		Body string `json:"body"`
	}
	if !httpx.Bind(c, &in) {
		return
	}
	m, err := h.Chat.Send(c.Request.Context(), middleware.UserID(c), id, in.Body)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, m)
}

func (h *Handlers) MarkRead(c *gin.Context) {
	id, ok := httpx.ParamUUID(c, "id")
	if !ok {
		return
	}
	if err := h.Chat.MarkRead(c.Request.Context(), middleware.UserID(c), id); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}

// ---- safety ----

func (h *Handlers) Block(c *gin.Context) {
	id, ok := httpx.ParamUUID(c, "id")
	if !ok {
		return
	}
	if err := h.Safety.Block(c.Request.Context(), middleware.UserID(c), id); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}

func (h *Handlers) Unblock(c *gin.Context) {
	id, ok := httpx.ParamUUID(c, "id")
	if !ok {
		return
	}
	if err := h.Safety.Unblock(c.Request.Context(), middleware.UserID(c), id); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}

func (h *Handlers) Blocked(c *gin.Context) {
	res, err := h.Safety.Blocked(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": res})
}

func (h *Handlers) Report(c *gin.Context) {
	id, ok := httpx.ParamUUID(c, "id")
	if !ok {
		return
	}
	var in struct {
		Reason  string `json:"reason"`
		Details string `json:"details"`
	}
	if !httpx.Bind(c, &in) {
		return
	}
	if err := h.Safety.Report(c.Request.Context(), middleware.UserID(c), id, in.Reason, in.Details); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}

// ---- billing ----

func (h *Handlers) Plans(c *gin.Context) {
	plans, err := h.Billing.Plans(c.Request.Context())
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": plans, "entitlements": h.Billing.Entitlements(c.Request.Context(), middleware.UserID(c))})
}

func (h *Handlers) Checkout(c *gin.Context) {
	var in struct {
		Plan     string `json:"plan"`
		Provider string `json:"provider"`
	}
	if !httpx.Bind(c, &in) {
		return
	}
	res, err := h.Billing.Checkout(c.Request.Context(), middleware.UserID(c), in.Plan, in.Provider)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, res)
}

func (h *Handlers) Entitlements(c *gin.Context) {
	httpx.OK(c, h.Billing.Entitlements(c.Request.Context(), middleware.UserID(c)))
}

func (h *Handlers) StripeWebhook(c *gin.Context) {
	body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20))
	if err != nil {
		httpx.Fail(c, apperr.BadRequest("invalid_body", "bad body"))
		return
	}
	if err := h.Billing.HandleStripeWebhook(c.Request.Context(), body, c.GetHeader("Stripe-Signature")); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, gin.H{"received": true})
}

var _ = time.Second
