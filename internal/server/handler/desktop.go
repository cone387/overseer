package handler

import (
	"crypto/rand"
	"encoding/hex"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/overseer/overseer/internal/config"
	"github.com/overseer/overseer/internal/model"
	"github.com/overseer/overseer/internal/store"
)

// PendingLink represents a desktop link request waiting for user confirmation.
type PendingLink struct {
	Code      string
	Name      string
	ExpiresAt time.Time
	Confirmed bool
	Device    *model.Device
}

// DesktopHandler handles desktop client self-registration and desktop-link auth flow.
type DesktopHandler struct {
	store       store.Store
	config      config.DesktopConfig
	pendingMu   sync.RWMutex
	pendingLinks map[string]*PendingLink
}

// NewDesktopHandler creates a new DesktopHandler.
func NewDesktopHandler(s store.Store, cfg config.DesktopConfig) *DesktopHandler {
	return &DesktopHandler{
		store:        s,
		config:       cfg,
		pendingLinks: make(map[string]*PendingLink),
	}
}

// RegisterRoutes registers desktop routes that do NOT require authentication.
func (h *DesktopHandler) RegisterRoutes(engine *gin.Engine) {
	engine.POST("/api/devices/desktop-register", h.HandleRegister)
	engine.POST("/api/devices/desktop-link", h.HandleLink)
	engine.GET("/api/devices/desktop-poll", h.HandlePoll)
}

// RegisterProtectedRoutes registers desktop routes that require authentication.
func (h *DesktopHandler) RegisterProtectedRoutes(rg *gin.RouterGroup) {
	rg.POST("/devices/desktop-link/confirm", h.HandleConfirm)
}

// ─── Desktop Link Flow ─────────────────────────────────────────────────────────

type desktopLinkRequest struct {
	Name string `json:"name"`
}

// HandleLink handles POST /api/devices/desktop-link.
// Generates a short-lived link code for the desktop client to present to the user.
func (h *DesktopHandler) HandleLink(c *gin.Context) {
	var req desktopLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Name = ""
	}

	name := req.Name
	if name == "" {
		name = "Desktop Client"
	}

	// Check max devices limit
	if h.config.MaxDevices > 0 {
		count, err := h.store.CountDevicesByType(model.DeviceTypeDesktop)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": "failed to check device count",
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

	code := generateLinkCode()

	h.pendingMu.Lock()
	h.pendingLinks[code] = &PendingLink{
		Code:      code,
		Name:      name,
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}
	h.pendingMu.Unlock()

	// Schedule cleanup
	go func() {
		time.Sleep(5 * time.Minute)
		h.pendingMu.Lock()
		delete(h.pendingLinks, code)
		h.pendingMu.Unlock()
	}()

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"link_code": code,
		},
	})
}

// HandlePoll handles GET /api/devices/desktop-poll?code=xxx.
// Returns 202 if pending, 200 with device if confirmed, 404 if not found/expired.
func (h *DesktopHandler) HandlePoll(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "code parameter is required",
			"data":    nil,
		})
		return
	}

	h.pendingMu.RLock()
	link, exists := h.pendingLinks[code]
	h.pendingMu.RUnlock()

	if !exists || time.Now().After(link.ExpiresAt) {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "link code not found or expired",
			"data":    nil,
		})
		return
	}

	if !link.Confirmed {
		c.JSON(http.StatusAccepted, gin.H{
			"code":    202,
			"message": "pending",
			"status":  "pending",
		})
		return
	}

	// Confirmed — return device info and clean up
	device := link.Device
	h.pendingMu.Lock()
	delete(h.pendingLinks, code)
	h.pendingMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    device,
	})
}

type desktopConfirmRequest struct {
	Code string `json:"code" binding:"required"`
}

// HandleConfirm handles POST /api/devices/desktop-link/confirm (authenticated).
// Confirms a pending desktop link code and creates the device.
func (h *DesktopHandler) HandleConfirm(c *gin.Context) {
	var req desktopConfirmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "code is required",
			"data":    nil,
		})
		return
	}

	h.pendingMu.Lock()
	link, exists := h.pendingLinks[req.Code]
	if !exists || time.Now().After(link.ExpiresAt) {
		h.pendingMu.Unlock()
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "link code not found or expired",
			"data":    nil,
		})
		return
	}

	if link.Confirmed {
		h.pendingMu.Unlock()
		c.JSON(http.StatusOK, gin.H{
			"code":    200,
			"message": "already confirmed",
			"data":    link.Device,
		})
		return
	}

	// Generate device key
	keyBytes := make([]byte, 16)
	if _, err := rand.Read(keyBytes); err != nil {
		h.pendingMu.Unlock()
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to generate device key",
			"data":    nil,
		})
		return
	}
	deviceKey := "dk_" + hex.EncodeToString(keyBytes)

	now := time.Now()
	device := &model.Device{
		ID:        uuid.New().String(),
		Name:      link.Name,
		DeviceKey: deviceKey,
		Type:      model.DeviceTypeDesktop,
		IsDefault: false,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := h.store.CreateDevice(device); err != nil {
		h.pendingMu.Unlock()
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to create device: " + err.Error(),
			"data":    nil,
		})
		return
	}

	link.Confirmed = true
	link.Device = device
	h.pendingMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    device,
	})
}

// ─── Legacy Token-based Registration ───────────────────────────────────────────

type desktopRegisterRequest struct {
	Name  string `json:"name"`
	Token string `json:"token" binding:"required"`
}

// HandleRegister handles POST /api/devices/desktop-register.
func (h *DesktopHandler) HandleRegister(c *gin.Context) {
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

	if req.Token != h.config.RegisterToken {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "invalid registration token",
			"data":    nil,
		})
		return
	}

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

// ─── Helpers ────────────────────────────────────────────────────────────────────

const linkCodeChars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func generateLinkCode() string {
	code := make([]byte, 6)
	for i := range code {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(linkCodeChars))))
		code[i] = linkCodeChars[n.Int64()]
	}
	return string(code)
}
