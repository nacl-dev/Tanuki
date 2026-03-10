package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ─── Response envelope ────────────────────────────────────────────────────────

// envelope is the standard JSON response wrapper for all API endpoints.
type envelope struct {
	Data  interface{} `json:"data"`
	Error string      `json:"error,omitempty"`
	Meta  *Meta       `json:"meta,omitempty"`
}

// Meta carries pagination metadata.
type Meta struct {
	Page  int `json:"page,omitempty"`
	Total int `json:"total"`
}

func respondOK(c *gin.Context, data interface{}, meta *Meta) {
	c.JSON(http.StatusOK, envelope{Data: data, Meta: meta})
}

func respondError(c *gin.Context, status int, msg string) {
	c.AbortWithStatusJSON(status, envelope{Error: msg})
}

// itoa converts an integer to its decimal string representation.
// Used internally for building parameterized SQL queries.
func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
