package handler

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/overseer/overseer/internal/config"
	"github.com/overseer/overseer/internal/model"
	"github.com/overseer/overseer/internal/store"
)

// DesktopHandler handles desktop client self-registration.
type DesktopHandler struct {
	store  store.Store
	config config.DesktopConfig
}

// NewDesktopHandler creates a new DesktopHandler.
func NewDesktopHandler(s store.Store, cfg config.DesktopConfig) *DesktopHandler {
	return &DesktopHandler{store: s, config: cfg}
}

// RegisterRoutes registers the desktop registration route on the given engine (no auth middleware).
func (h *DesktopHandler) RegisterRoutes(engine *gin.Engine) {
	engine.POST("/api/devices/desktop-register", h.HandleRegister)
}

// desktopRegisterRequest is the request body for desktop self-registration.
type desktopRegisterRequest struct {
	Name  string `json:"name"`
	Token string `json:"token" binding:"required"`
}

// HandleRegister handles POST /api/devices/desktop-register.
// Requires a valid register_token from config for security.
func (h *DesktopHandler) HandleRegister(c *gin.Context) {
	// Check if desktop registration is enabled
	if h.config.RegisterToken == "" {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    403,
			"message": "desktop registration is disabled (no register_token configured)",
			"data":    nil,
		})
		return
	}

	var req desktopRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "validation error: token is required",
			"data":    nil,
		})
		return
	}

	// Validate the registration token
	if req.Token != h.config.RegisterToken {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "invalid registration token",
			"data":    nil,
		})
		return
	}

	// Check max devices limit
	if h.config.MaxDevices > 0 {
		count, err := h.store.CountDevicesByType(model.DeviceTypeDesktop)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": "failed to check device count: " + err.Error(),
				"data":    nil,
			})
			return
		}
		if count >= h.config.MaxDevices {
			c.JSON(http.StatusForbidden, gin.H{
				"code":    403,
				"message": "maximum number of desktop devices reached",
				"data":    nil,
			})
			return
		}
	}

	// Generate device key: dk_ + 32 random hex characters
	keyBytes := make([]byte, 16)
	if _, err := rand.Read(keyBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to generate device key",
			"data":    nil,
		})
		return
	}
	deviceKey := "dk_" + hex.EncodeToString(keyBytes)

	// Default name
	name := req.Name
	if name == "" {
		name = "Desktop Client"
	}

	now := time.Now()
	device := &model.Device{
		ID:        uuid.New().String(),
		Name:      name,
		DeviceKey: deviceKey,
		Type:      model.DeviceTypeDesktop,
		IsDefault: false,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := h.store.CreateDevice(device); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to register device: " + err.Error(),
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    device,
	})
}
