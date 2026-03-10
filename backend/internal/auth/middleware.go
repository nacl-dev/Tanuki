package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	contextUserID = "userID"
	contextRole   = "role"
)

// AuthRequired returns a Gin middleware that validates a Bearer JWT token from
// the Authorization header. On success it sets "userID" and "role" in the
// context. On failure it aborts with 401 Unauthorized.
func AuthRequired(secretKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authorization header required"})
			return
		}

		tokenStr := strings.TrimPrefix(header, "Bearer ")
		claims, err := ValidateToken(tokenStr, secretKey)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		c.Set(contextUserID, claims.Subject)
		c.Set(contextRole, claims.Role)
		c.Next()
	}
}

// AdminRequired returns a Gin middleware that checks the role stored in the
// context (set by AuthRequired) is "admin". Must be used after AuthRequired.
func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get(contextRole)
		if role != string(roleAdmin) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin access required"})
			return
		}
		c.Next()
	}
}

// roleAdmin is the internal constant used inside this package.
const roleAdmin = "admin"
