package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// APIKeyAuth returns a Gin middleware that validates the X-API-Key header.
// Requests to /health are exempt from authentication.
func APIKeyAuth(apiKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// /health is exempt from auth (handled outside this middleware group,
		// but included as a safety check)
		if c.Request.URL.Path == "/health" {
			c.Next()
			return
		}

		key := c.GetHeader("X-API-Key")
		if key == "" || key != apiKey {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "unauthorized: invalid or missing API key",
			})
			return
		}

		c.Next()
	}
}
