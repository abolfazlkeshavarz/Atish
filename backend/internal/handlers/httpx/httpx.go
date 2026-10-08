// Package httpx holds tiny HTTP helpers shared by handlers and middleware.
package httpx

import (
	"log"
	"net/http"
	"strconv"

	"atish/internal/apperr"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type errBody struct {
	Error *apperr.Error `json:"error"`
}

// Fail writes a JSON error. Unknown errors become a generic 500 (details are logged, never leaked).
func Fail(c *gin.Context, err error) {
	if e, ok := apperr.As(err); ok {
		c.AbortWithStatusJSON(e.Status, errBody{e})
		return
	}
	log.Printf("internal error: %s %s: %v", c.Request.Method, c.Request.URL.Path, err)
	c.AbortWithStatusJSON(http.StatusInternalServerError, errBody{apperr.New(500, "internal_error", "Something went wrong")})
}

func OK(c *gin.Context, v any) { c.JSON(http.StatusOK, v) }

func Created(c *gin.Context, v any) { c.JSON(http.StatusCreated, v) }

func NoContent(c *gin.Context) { c.Status(http.StatusNoContent) }

// Bind decodes a JSON body with a size cap and rejects malformed input.
func Bind(c *gin.Context, dst any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	if err := c.ShouldBindJSON(dst); err != nil {
		Fail(c, apperr.BadRequest("invalid_body", "Invalid request body"))
		return false
	}
	return true
}

func ParamUUID(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		Fail(c, apperr.BadRequest("invalid_id", "Invalid id"))
		return uuid.Nil, false
	}
	return id, true
}

func QueryInt(c *gin.Context, name string, def int) int {
	if v, err := strconv.Atoi(c.Query(name)); err == nil {
		return v
	}
	return def
}

func QueryInt64(c *gin.Context, name string) int64 {
	v, _ := strconv.ParseInt(c.Query(name), 10, 64)
	return v
}

// Page returns a sane limit/offset from ?per=&page=.
func Page(c *gin.Context) (limit, offset int) {
	limit = QueryInt(c, "per", 25)
	if limit < 1 || limit > 100 {
		limit = 25
	}
	page := QueryInt(c, "page", 1)
	if page < 1 {
		page = 1
	}
	return limit, (page - 1) * limit
}
