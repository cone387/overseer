package handler

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/overseer/overseer/internal/config"
	"github.com/overseer/overseer/internal/model"
	"github.com/overseer/overseer/internal/store"
)

// ─── Desktop Link (Polling Auth Flow) ────────────────────────────────────────

// linkSession represents a pending desktop authorization session.
type linkSession struct {
	Code      string
	Name      string
	CreatedAt time.Time
	// Filled after user confirms
	Confirmed bool
	Device    *model.Device
}

// linkStore manages pending link sessions in memory.
type linkStore struct {
	mu       sync.Mutex
	sessions map[string]*linkSession
}

func newLinkStore() *linkStore {
	ls := &linkStore{sessions: make(map[string]*linkSession)}
	// Cleanup expired sessions every minute
	go func() {
		for {
			time.Sleep(time.Minute)
			ls.cleanup()
		}
	}()
	return ls
}

func (ls *linkStore) create(name string) *linkSession {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	code := generateCode()
	session := &linkSession{
		Code:      code,
		Name:      name,
		CreatedAt: time.Now(),
	}
	ls.sessions[code] = session
	return session
}

func (ls *linkStore) get(code string) *linkSession {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	return ls.sessions[code]
}

func (ls *linkStore) confirm(code string, device *model.Device) bool {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	s, ok := ls.sessions[code]
	if !ok {
		return false
	}
	s.Confirmed = true
	s.Device = device
	return true
}

func (ls *linkStore) remove(code string) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	delete(ls.sessions, code)
}

func (ls *linkStore) cleanup() {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	cutoff := time.Now().Add(-10 * time.Minute)
	for code, s := range ls.sessions {
		if s.CreatedAt.Before(cutoff) {
			delete(ls.sessions, code)
		}
	}
}

// generateCode creates a short 6-character uppercase code for display.
func generateCode() string {
	b := make([]byte, 3)
	rand.Read(b)
	return fmt.Sprintf("%s", hex.EncodeToString(b))
}

// ─── Desktop Handler ─────────────────────────────────────────────────────────

// DesktopHandler handles desktop client self-registration.
type DesktopHandler struct {
	store     store.Store
	config    config.DesktopConfig
	linkStore *linkStore
}

// NewDesktopHandler creates a new DesktopHandler.
func NewDesktopHandler(s store.Store, cfg config.DesktopConfig) *DesktopHandler {
	return &DesktopHandler{store: s, config: cfg, linkStore: newLinkStore()}
}

// RegisterRoutes registers the desktop registration route on the given engine (no auth middleware).
func (h *DesktopHandler) RegisterRoutes(engine *gin.Engine) {
	engine.POST("/api/devices/desktop-register", h.HandleRegister)
	// Polling auth flow (no auth required for these)
	engine.POST("/api/devices/desktop-link", h.HandleLink)
	engine.GET("/api/devices/desktop-poll", h.HandlePoll)
}

// RegisterProtectedRoutes registers routes that require authentication.
func (h *DesktopHandler) RegisterProtectedRoutes(rg *gin.RouterGroup) {
	rg.POST("/devices/desktop-confirm", h.HandleConfirm)
}

// ─── Polling Auth Flow Handlers ──────────────────────────────────────────────

// HandleLink handles POST /api/devices/desktop-link.
// Desktop client calls this to initiate the auth flow. Returns a code and link URL.
func (h *DesktopHandler) HandleLink(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	c.ShouldBindJSON(&req)

	if req.Name == "" {
		req.Name = "Desktop Client"
	}

	session := h.linkStore.create(req.Name)

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"link_code":  session.Code,
			"expires_in": 600, // 10 minutes
		},
	})
}

// HandlePoll handles GET /api/devices/desktop-poll?code=xxx.
// Desktop client polls this until the user confirms authorization.
func (h *DesktopHandler) HandlePoll(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "code is required",
			"data":    nil,
		})
		return
	}

	session := h.linkStore.get(code)
	if session == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "link code not found or expired",
			"data":    nil,
		})
		return
	}

	// Check expiry (10 minutes)
	if time.Since(session.CreatedAt) > 10*time.Minute {
		h.linkStore.remove(code)
		c.JSON(http.StatusGone, gin.H{
			"code":    410,
			"message": "link code expired",
			"data":    nil,
		})
		return
	}

	if !session.Confirmed {
		c.JSON(http.StatusAccepted, gin.H{
			"code":    202,
			"message": "pending",
			"data":    nil,
		})
		return
	}

	// Confirmed! Return device info and clean up
	device := session.Device
	h.linkStore.remove(code)

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    device,
	})
}

// HandleConfirm handles POST /api/devices/desktop-confirm (requires auth).
// Web user calls this to confirm the desktop link and register the device.
func (h *DesktopHandler) HandleConfirm(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "code is required",
			"data":    nil,
		})
		return
	}

	session := h.linkStore.get(req.Code)
	if session == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "link code not found or expired",
			"data":    nil,
		})
		return
	}

	if time.Since(session.CreatedAt) > 10*time.Minute {
		h.linkStore.remove(req.Code)
		c.JSON(http.StatusGone, gin.H{
			"code":    410,
			"message": "link code expired",
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

	// Register the device
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

	now := time.Now()
	device := &model.Device{
		ID:        uuid.New().String(),
		Name:      session.Name,
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

	// Mark session as confirmed
	h.linkStore.confirm(req.Code, device)

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    device,
	})
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
