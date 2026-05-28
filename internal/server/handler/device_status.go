package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/overseer/overseer/internal/store"
	"github.com/overseer/overseer/internal/ws"
)

// DeviceStatusHandler handles the device online status API.
type DeviceStatusHandler struct {
	store store.Store
	hub   *ws.Hub
}

// DeviceStatusResponse represents a device with its online status.
type DeviceStatusResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	DeviceKey string `json:"device_key"`
	IsDefault bool   `json:"is_default"`
	Online    bool   `json:"online"`
}

// NewDeviceStatusHandler creates a new DeviceStatusHandler.
func NewDeviceStatusHandler(s store.Store, hub *ws.Hub) *DeviceStatusHandler {
	return &DeviceStatusHandler{store: s, hub: hub}
}

// RegisterRoutes registers the device status route.
func (h *DeviceStatusHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/devices/status", h.GetStatus)
}

// GetStatus handles GET /api/devices/status.
// Returns all devices with their online/offline status.
func (h *DeviceStatusHandler) GetStatus(c *gin.Context) {
	devices, err := h.store.ListDevices()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to list devices: " + err.Error(),
			"data":    nil,
		})
		return
	}

	// Get currently online device keys from the Hub
	onlineKeys := make(map[string]bool)
	for _, key := range h.hub.OnlineDeviceKeys() {
		onlineKeys[key] = true
	}

	// Build response with online status
	result := make([]DeviceStatusResponse, 0, len(devices))
	for _, d := range devices {
		result = append(result, DeviceStatusResponse{
			ID:        d.ID,
			Name:      d.Name,
			Type:      d.Type,
			DeviceKey: d.DeviceKey,
			IsDefault: d.IsDefault,
			Online:    onlineKeys[d.DeviceKey],
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    result,
	})
}
