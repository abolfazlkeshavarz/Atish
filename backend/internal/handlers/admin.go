package handlers

import (
	"encoding/json"
	"strconv"

	"atish/internal/apperr"
	"atish/internal/handlers/httpx"
	"atish/internal/middleware"
	"atish/internal/repository"

	"github.com/gin-gonic/gin"
)

func actor(c *gin.Context) (string, string) {
	return c.GetString(middleware.CtxActor), c.GetString(middleware.CtxRole)
}

func (h *Handlers) AdminWhoAmI(c *gin.Context) {
	a, role := actor(c)
	httpx.OK(c, gin.H{"actor": a, "role": role})
}

func (h *Handlers) AdminStats(c *gin.Context) {
	res, err := h.Admin.Stats(c.Request.Context())
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.Data(200, "application/json", res)
}

func (h *Handlers) AdminUsers(c *gin.Context) {
	limit, offset := httpx.Page(c)
	rows, total, err := h.Admin.Users(c.Request.Context(), repository.UserQuery{
		Q: c.Query("q"), Status: c.Query("status"), Role: c.Query("role"), Premium: c.Query("premium"), Limit: limit, Offset: offset,
	})
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": rows, "total": total})
}

func (h *Handlers) AdminUser(c *gin.Context) {
	id, ok := httpx.ParamUUID(c, "id")
	if !ok {
		return
	}
	d, err := h.Admin.UserDetail(c.Request.Context(), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, d)
}

func (h *Handlers) AdminUpdateUser(c *gin.Context) {
	id, ok := httpx.ParamUUID(c, "id")
	if !ok {
		return
	}
	var in map[string]any
	if !httpx.Bind(c, &in) {
		return
	}
	a, role := actor(c)
	if err := h.Admin.UpdateUser(c.Request.Context(), a, role, id, in); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}

func (h *Handlers) AdminEditProfile(c *gin.Context) {
	id, ok := httpx.ParamUUID(c, "id")
	if !ok {
		return
	}
	var in struct {
		DisplayName *string `json:"display_name"`
		Bio         *string `json:"bio"`
	}
	if !httpx.Bind(c, &in) {
		return
	}
	a, _ := actor(c)
	if err := h.Admin.EditProfileText(c.Request.Context(), a, id, in.DisplayName, in.Bio); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}

func (h *Handlers) AdminDeleteUser(c *gin.Context) {
	id, ok := httpx.ParamUUID(c, "id")
	if !ok {
		return
	}
	a, _ := actor(c)
	if err := h.Admin.DeleteUser(c.Request.Context(), a, id); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}

func (h *Handlers) AdminGrantPremium(c *gin.Context) {
	id, ok := httpx.ParamUUID(c, "id")
	if !ok {
		return
	}
	var in struct {
		Plan string `json:"plan"`
		Days int    `json:"days"`
	}
	if !httpx.Bind(c, &in) {
		return
	}
	a, _ := actor(c)
	if err := h.Admin.GrantPremium(c.Request.Context(), a, id, in.Plan, in.Days); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}

func (h *Handlers) AdminRevokePremium(c *gin.Context) {
	id, ok := httpx.ParamUUID(c, "id")
	if !ok {
		return
	}
	a, _ := actor(c)
	if err := h.Admin.RevokePremium(c.Request.Context(), a, id); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}

func (h *Handlers) AdminRemovePhoto(c *gin.Context) {
	id, ok := httpx.ParamUUID(c, "id")
	if !ok {
		return
	}
	a, _ := actor(c)
	if err := h.Admin.RemovePhoto(c.Request.Context(), a, id); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}

func (h *Handlers) AdminRecentPhotos(c *gin.Context) {
	limit, offset := httpx.Page(c)
	res, err := h.Admin.RecentPhotos(c.Request.Context(), limit, offset)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": res})
}

func (h *Handlers) AdminReports(c *gin.Context) {
	limit, offset := httpx.Page(c)
	res, err := h.Admin.Reports(c.Request.Context(), c.Query("status"), limit, offset)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": res})
}

func (h *Handlers) AdminReport(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, apperr.BadRequest("invalid_id", "Invalid id"))
		return
	}
	a, _ := actor(c)
	res, err := h.Admin.Report(c.Request.Context(), a, id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, res)
}

func (h *Handlers) AdminResolveReport(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, apperr.BadRequest("invalid_id", "Invalid id"))
		return
	}
	var in struct {
		Status     string `json:"status"`
		Resolution string `json:"resolution"`
		Action     string `json:"action"` // none | suspend | ban
	}
	if !httpx.Bind(c, &in) {
		return
	}
	a, role := actor(c)
	if err := h.Admin.ResolveReport(c.Request.Context(), a, role, id, in.Status, in.Resolution, in.Action); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}

func (h *Handlers) AdminCatalogList(c *gin.Context) {
	limit, offset := httpx.Page(c)
	if l := httpx.QueryInt(c, "per", 0); l > 0 && l <= 500 {
		limit = l
	}
	items, total, err := h.Admin.CatalogList(c.Request.Context(), c.Param("resource"), c.Query("q"), limit, offset)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items, "total": total})
}

func (h *Handlers) AdminCatalogCreate(c *gin.Context) {
	var in map[string]any
	if !httpx.Bind(c, &in) {
		return
	}
	a, _ := actor(c)
	res, err := h.Admin.CatalogCreate(c.Request.Context(), a, c.Param("resource"), in)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.Data(201, "application/json", res)
}

func (h *Handlers) AdminCatalogUpdate(c *gin.Context) {
	var in map[string]any
	if !httpx.Bind(c, &in) {
		return
	}
	a, _ := actor(c)
	res, err := h.Admin.CatalogUpdate(c.Request.Context(), a, c.Param("resource"), c.Param("id"), in)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.Data(200, "application/json", res)
}

func (h *Handlers) AdminCatalogDelete(c *gin.Context) {
	a, _ := actor(c)
	if err := h.Admin.CatalogDelete(c.Request.Context(), a, c.Param("resource"), c.Param("id")); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}

func (h *Handlers) AdminSettings(c *gin.Context) {
	httpx.OK(c, h.Admin.Settings(c.Request.Context()))
}

func (h *Handlers) AdminSetSetting(c *gin.Context) {
	var in struct {
		Value json.RawMessage `json:"value"`
	}
	if !httpx.Bind(c, &in) {
		return
	}
	a, _ := actor(c)
	if err := h.Admin.SetSetting(c.Request.Context(), a, c.Param("key"), in.Value); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}

func (h *Handlers) AdminPayments(c *gin.Context) {
	limit, offset := httpx.Page(c)
	res, err := h.Admin.Payments(c.Request.Context(), c.Query("status"), limit, offset)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": res})
}

func (h *Handlers) AdminSubscriptions(c *gin.Context) {
	limit, offset := httpx.Page(c)
	res, err := h.Admin.Subscriptions(c.Request.Context(), c.Query("status"), limit, offset)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": res})
}

func (h *Handlers) AdminAudit(c *gin.Context) {
	limit, offset := httpx.Page(c)
	res, err := h.Admin.AuditLog(c.Request.Context(), limit, offset)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": res})
}

func (h *Handlers) AdminBroadcast(c *gin.Context) {
	var in struct {
		Text string `json:"text"`
	}
	if !httpx.Bind(c, &in) {
		return
	}
	a, _ := actor(c)
	n, err := h.Admin.Broadcast(c.Request.Context(), a, in.Text)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, gin.H{"recipients": n})
}
