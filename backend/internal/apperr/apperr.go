// Package apperr defines errors that carry an HTTP status and a stable,
// machine-readable code so every client (web, Android, iOS) can react to them.
package apperr

import (
	"errors"
	"net/http"
)

type Error struct {
	Status  int            `json:"-"`
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data,omitempty"`
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

func (e *Error) With(k string, v any) *Error {
	c := *e
	c.Data = map[string]any{k: v}
	return &c
}

func New(status int, code, msg string) *Error {
	return &Error{Status: status, Code: code, Message: msg}
}

func BadRequest(code, msg string) *Error      { return New(http.StatusBadRequest, code, msg) }
func Unauthorized(msg string) *Error          { return New(http.StatusUnauthorized, "unauthorized", msg) }
func Forbidden(code, msg string) *Error       { return New(http.StatusForbidden, code, msg) }
func NotFound(msg string) *Error              { return New(http.StatusNotFound, "not_found", msg) }
func Conflict(code, msg string) *Error        { return New(http.StatusConflict, code, msg) }
func PaymentRequired(code, msg string) *Error { return New(http.StatusPaymentRequired, code, msg) }
func TooMany(msg string) *Error               { return New(http.StatusTooManyRequests, "rate_limited", msg) }

var (
	ErrNotFound = NotFound("resource not found")
)

func As(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}
