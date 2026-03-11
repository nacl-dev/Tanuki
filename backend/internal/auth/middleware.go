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

const authCookieName = "tanuki_auth"

type authEnvelope struct {
	Error     string `json:"error"`
	ErrorCode string `json:"error_code,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

// AuthRequired returns a Gin middleware that validates a JWT from either the
// Authorization header or the auth cookie. On success it sets "userID" and
// "role" in the context. On failure it aborts with 401 Unauthorized.
func AuthRequired(secretKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := ""
		header := c.GetHeader("Authorization")
		if strings.HasPrefix(header, "Bearer ") {
			tokenStr = strings.TrimPrefix(header, "Bearer ")
		} else if cookie, err := c.Cookie(authCookieName); err == nil {
			tokenStr = cookie
		}
		if tokenStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, authEnvelope{
				Error:     "authorization required",
				ErrorCode: "unauthorized",
				RequestID: c.GetString("requestID"),
			})
			return
		}

		claims, err := ValidateToken(tokenStr, secretKey)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, authEnvelope{
				Error:     "invalid or expired token",
				ErrorCode: "unauthorized",
				RequestID: c.GetString("requestID"),
			})
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
			c.AbortWithStatusJSON(http.StatusForbidden, authEnvelope{
				Error:     "admin access required",
				ErrorCode: "forbidden",
				RequestID: c.GetString("requestID"),
			})
			return
		}
		c.Next()
	}
}

// roleAdmin is the internal constant used inside this package.
const roleAdmin = "admin"
