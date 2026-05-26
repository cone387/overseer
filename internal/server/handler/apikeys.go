package handler

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/overseer/overseer/internal/model"
	"github.com/overseer/overseer/internal/server/middleware"
	"github.com/overseer/overseer/internal/store"
	"golang.org/x/crypto/bcrypt"
)

// APIKeysHandler handles API key management endpoints.
type APIKeysHandler struct {
	store store.Store
}

// NewAPIKeysHandler creates a new APIKeysHandler.
func NewAPIKeysHandler(store store.Store) *APIKeysHandler {
	return &APIKeysHandler{store: store}
}

// RegisterRoutes registers API key routes on the given router group.
// All routes require JWT authentication.
func (h *APIKeysHandler) RegisterRoutes(api *gin.RouterGroup) {
	api.GET("/keys", h.List)
	api.POST("/keys", h.Create)
	api.DELETE("/keys/:id", h.Delete)
}

// List returns all API keys for the authenticated user (without the actual key value).
func (h *APIKeysHandler) List(c *gin.Context) {
	user := middleware.GetUserFromContext(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized"})
		return
	}

	keys, err := h.store.ListAPIKeys(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "failed to list API keys"})
		return
	}

	if keys == nil {
		keys = []model.APIKey{}
	}

	c.JSON(http.StatusOK, gin.H{"keys": keys})
}

type createKeyRequest struct {
	Name string `json:"name" binding:"required"`
}

// Create generates a new API key. The full key is returned only once.
func (h *APIKeysHandler) Create(c *gin.Context) {
	var req createKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid request body: name is required"})
		return
	}

	user := middleware.GetUserFromContext(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized"})
		return
	}

	// Generate random 32-byte key
	rawBytes := make([]byte, 32)
	if _, err := rand.Read(rawBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "failed to generate key"})
		return
	}
	rawKey := base64.URLEncoding.EncodeToString(rawBytes)

	// Hash the key for storage
	hash, err := bcrypt.GenerateFromPassword([]byte(rawKey), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "failed to hash key"})
		return
	}

	// Take first 8 chars as prefix for display
	prefix := rawKey[:8]

	now := time.Now()
	apiKey := &model.APIKey{
		ID:        uuid.New().String(),
		Name:      req.Name,
		KeyHash:   string(hash),
		Prefix:    prefix,
		UserID:    user.ID,
		CreatedAt: now,
	}

	if err := h.store.CreateAPIKey(apiKey); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "failed to save API key"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":         apiKey.ID,
		"name":       apiKey.Name,
		"key":        rawKey,
		"prefix":     prefix,
		"created_at": apiKey.CreatedAt,
	})
}

// Delete revokes/deletes an API key.
func (h *APIKeysHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "key id is required"})
		return
	}

	if err := h.store.DeleteAPIKey(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "API key not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "API key deleted"})
}
