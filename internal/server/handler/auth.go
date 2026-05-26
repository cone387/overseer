package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/overseer/overseer/internal/model"
	"github.com/overseer/overseer/internal/server/middleware"
	"github.com/overseer/overseer/internal/store"
	"golang.org/x/crypto/bcrypt"
)

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	store     store.Store
	jwtSecret string
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(store store.Store, jwtSecret string) *AuthHandler {
	return &AuthHandler{
		store:     store,
		jwtSecret: jwtSecret,
	}
}

// RegisterRoutes registers auth routes on the given router group.
// These routes do NOT require authentication.
func (h *AuthHandler) RegisterRoutes(r *gin.Engine) {
	auth := r.Group("/api/auth")
	auth.GET("/status", h.Status)
	auth.POST("/register", h.Register)
	auth.POST("/login", h.Login)
	auth.POST("/logout", h.Logout)
}

// RegisterProtectedRoutes registers auth routes that require authentication.
func (h *AuthHandler) RegisterProtectedRoutes(api *gin.RouterGroup) {
	api.POST("/auth/change-password", h.ChangePassword)
	api.GET("/auth/me", h.Me)
}

// Status returns whether the system is initialized (has users) and if the current request is authenticated.
func (h *AuthHandler) Status(c *gin.Context) {
	count, err := h.store.GetUserCount()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "failed to check user count"})
		return
	}

	authenticated := false
	tokenStr := ""
	if cookie, err := c.Cookie("overseer_token"); err == nil && cookie != "" {
		tokenStr = cookie
	} else if auth := c.GetHeader("Authorization"); len(auth) > 7 {
		tokenStr = auth[7:]
	}
	if tokenStr != "" {
		claims := &middleware.JWTClaims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
			return []byte(h.jwtSecret), nil
		})
		if err == nil && token.Valid {
			authenticated = true
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"initialized":   count > 0,
		"authenticated": authenticated,
	})
}

type registerRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Register handles first-time user registration. Only works if no users exist.
func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid request body"})
		return
	}

	if len(req.Username) < 3 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "username must be at least 3 characters"})
		return
	}
	if len(req.Password) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "password must be at least 6 characters"})
		return
	}

	// Only allow registration if no users exist
	count, err := h.store.GetUserCount()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "failed to check user count"})
		return
	}
	if count > 0 {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "registration is disabled: a user already exists"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "failed to hash password"})
		return
	}

	now := time.Now()
	user := &model.User{
		ID:           uuid.New().String(),
		Username:     req.Username,
		PasswordHash: string(hash),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := h.store.CreateUser(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "failed to create user"})
		return
	}

	tokenStr, err := h.generateToken(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "failed to generate token"})
		return
	}

	h.setTokenCookie(c, tokenStr)
	c.JSON(http.StatusOK, gin.H{
		"token": tokenStr,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
		},
	})
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Login authenticates a user and returns a JWT token.
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid request body"})
		return
	}

	user, err := h.store.GetUserByUsername(req.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "internal error"})
		return
	}
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "invalid username or password"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "invalid username or password"})
		return
	}

	tokenStr, err := h.generateToken(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "failed to generate token"})
		return
	}

	h.setTokenCookie(c, tokenStr)
	c.JSON(http.StatusOK, gin.H{
		"token": tokenStr,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
		},
	})
}

// Logout clears the auth cookie.
func (h *AuthHandler) Logout(c *gin.Context) {
	c.SetCookie("overseer_token", "", -1, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

type changePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

// ChangePassword updates the authenticated user's password.
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid request body"})
		return
	}

	if len(req.NewPassword) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "new password must be at least 6 characters"})
		return
	}

	user := middleware.GetUserFromContext(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized"})
		return
	}

	// Get full user to verify old password
	fullUser, err := h.store.GetUserByUsername(user.Username)
	if err != nil || fullUser == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "internal error"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(fullUser.PasswordHash), []byte(req.OldPassword)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "old password is incorrect"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "failed to hash password"})
		return
	}

	if err := h.store.UpdateUserPassword(fullUser.ID, string(hash)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "failed to update password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "password updated"})
}

// Me returns the current authenticated user's info.
func (h *AuthHandler) Me(c *gin.Context) {
	user := middleware.GetUserFromContext(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":       user.ID,
		"username": user.Username,
	})
}

func (h *AuthHandler) generateToken(user *model.User) (string, error) {
	claims := middleware.JWTClaims{
		UserID:   user.ID,
		Username: user.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.jwtSecret))
}

func (h *AuthHandler) setTokenCookie(c *gin.Context, token string) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("overseer_token", token, 86400, "/", "", false, true)
}
