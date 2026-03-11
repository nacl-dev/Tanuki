package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// ─── Response envelope ────────────────────────────────────────────────────────

// envelope is the standard JSON response wrapper for all API endpoints.
type envelope struct {
	Data      interface{} `json:"data"`
	Error     string      `json:"error,omitempty"`
	ErrorCode string      `json:"error_code,omitempty"`
	RequestID string      `json:"request_id,omitempty"`
	Meta      *Meta       `json:"meta,omitempty"`
}

// Meta carries pagination metadata.
type Meta struct {
	Page  int `json:"page,omitempty"`
	Total int `json:"total"`
}

func respondOK(c *gin.Context, data interface{}, meta *Meta) {
	respondWithStatus(c, http.StatusOK, data, meta)
}

func respondAccepted(c *gin.Context, data interface{}, meta *Meta) {
	respondWithStatus(c, http.StatusAccepted, data, meta)
}

func respondWithStatus(c *gin.Context, status int, data interface{}, meta *Meta) {
	c.JSON(status, envelope{
		Data:      data,
		RequestID: requestIDFromContext(c),
		Meta:      meta,
	})
}

func respondError(c *gin.Context, status int, msg string) {
	c.AbortWithStatusJSON(status, envelope{
		Error:     msg,
		ErrorCode: defaultErrorCode(status),
		RequestID: requestIDFromContext(c),
	})
}

func requestIDFromContext(c *gin.Context) string {
	if c == nil {
		return ""
	}
	return c.GetString("requestID")
}

func defaultErrorCode(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "bad_request"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusConflict:
		return "conflict"
	default:
		if status >= 500 {
			return "internal_error"
		}
		return strings.ToLower(strings.ReplaceAll(http.StatusText(status), " ", "_"))
	}
}

// itoa converts an integer to its decimal string representation.
// Used internally for building parameterized SQL queries.
func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
