package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/overseer/overseer/internal/model"
	"github.com/overseer/overseer/internal/store"
)

// JWTClaims defines the claims stored in the JWT token.
type JWTClaims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// JWTAuth returns a Gin middleware that validates JWT tokens.
// It checks the "overseer_token" cookie first, then the Authorization: Bearer header.
func JWTAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/health" {
			c.Next()
			return
		}

		tokenStr := extractJWT(c)
		if tokenStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "unauthorized: missing token",
			})
			return
		}

		claims, err := validateJWT(tokenStr, secret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "unauthorized: invalid token",
			})
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Next()
	}
}

// APIKeyOrJWTAuth returns a Gin middleware that accepts either a valid JWT token
// or a valid X-API-Key header. The /health endpoint is always exempt.
func APIKeyOrJWTAuth(db store.Store, secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/health" {
			c.Next()
			return
		}

		// Try API Key first
		apiKey := c.GetHeader("X-API-Key")
		if apiKey != "" {
			key, err := db.ValidateAPIKey(apiKey)
			if err == nil && key != nil {
				c.Set("user_id", key.UserID)
				c.Set("auth_method", "api_key")
				c.Next()
				return
			}
		}

		// Try JWT
		tokenStr := extractJWT(c)
		if tokenStr != "" {
			claims, err := validateJWT(tokenStr, secret)
			if err == nil {
				c.Set("user_id", claims.UserID)
				c.Set("username", claims.Username)
				c.Set("auth_method", "jwt")
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "unauthorized: invalid or missing credentials",
		})
	}
}

// extractJWT extracts the JWT token from cookie or Authorization header.
func extractJWT(c *gin.Context) string {
	// Check cookie first
	if cookie, err := c.Cookie("overseer_token"); err == nil && cookie != "" {
		return cookie
	}

	// Check Authorization: Bearer header
	auth := c.GetHeader("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}

	return ""
}

// validateJWT parses and validates a JWT token string.
func validateJWT(tokenStr string, secret string) (*JWTClaims, error) {
	claims := &JWTClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, jwt.ErrSignatureInvalid
	}
	return claims, nil
}

// GetUserFromContext retrieves the authenticated user info from the gin context.
func GetUserFromContext(c *gin.Context) *model.User {
	userID, exists := c.Get("user_id")
	if !exists {
		return nil
	}
	username, _ := c.Get("username")
	usernameStr, _ := username.(string)
	return &model.User{
		ID:       userID.(string),
		Username: usernameStr,
	}
}
