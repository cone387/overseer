package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/overseer/overseer/internal/model"
	"github.com/overseer/overseer/internal/store"
)

// DeviceHandler handles device management API endpoints.
type DeviceHandler struct {
	store store.Store
}

// NewDeviceHandler creates a new DeviceHandler.
func NewDeviceHandler(s store.Store) *DeviceHandler {
	return &DeviceHandler{store: s}
}

// RegisterRoutes registers device routes on the given router group.
func (h *DeviceHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/devices", h.List)
	rg.POST("/devices", h.Create)
	rg.PUT("/devices/:id", h.Update)
	rg.DELETE("/devices/:id", h.Delete)
	rg.POST("/devices/:id/default", h.SetDefault)
}

// CreateDeviceRequest is the request body for creating a device.
type CreateDeviceRequest struct {
	Name      string `json:"name" binding:"required,min=1,max=100"`
	DeviceKey string `json:"device_key" binding:"required,min=1"`
	IsDefault bool   `json:"is_default"`
}

// UpdateDeviceRequest is the request body for updating a device.
type UpdateDeviceRequest struct {
	Name      *string `json:"name" binding:"omitempty,min=1,max=100"`
	DeviceKey *string `json:"device_key" binding:"omitempty,min=1"`
}

// List handles GET /api/devices.
func (h *DeviceHandler) List(c *gin.Context) {
	devices, err := h.store.ListDevices()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to list devices: " + err.Error(),
			"data":    nil,
		})
		return
	}

	if devices == nil {
		devices = []model.Device{}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    devices,
	})
}

// Create handles POST /api/devices.
func (h *DeviceHandler) Create(c *gin.Context) {
	var req CreateDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "validation error: " + err.Error(),
			"data":    nil,
		})
		return
	}

	now := time.Now()
	device := &model.Device{
		ID:        uuid.New().String(),
		Name:      req.Name,
		DeviceKey: req.DeviceKey,
		IsDefault: req.IsDefault,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := h.store.CreateDevice(device); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to create device: " + err.Error(),
			"data":    nil,
		})
		return
	}

	// If this device is set as default, update others
	if req.IsDefault {
		_ = h.store.SetDefaultDevice(device.ID)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    device,
	})
}

// Update handles PUT /api/devices/:id.
func (h *DeviceHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var req UpdateDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "validation error: " + err.Error(),
			"data":    nil,
		})
		return
	}

	// Fetch existing devices to find the one to update
	devices, err := h.store.ListDevices()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to fetch devices: " + err.Error(),
			"data":    nil,
		})
		return
	}

	var existing *model.Device
	for i := range devices {
		if devices[i].ID == id {
			existing = &devices[i]
			break
		}
	}

	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "device not found",
			"data":    nil,
		})
		return
	}

	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.DeviceKey != nil {
		existing.DeviceKey = *req.DeviceKey
	}

	if err := h.store.UpdateDevice(existing); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to update device: " + err.Error(),
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    nil,
	})
}

// Delete handles DELETE /api/devices/:id.
func (h *DeviceHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	if err := h.store.DeleteDevice(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to delete device: " + err.Error(),
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    nil,
	})
}

// SetDefault handles POST /api/devices/:id/default.
func (h *DeviceHandler) SetDefault(c *gin.Context) {
	id := c.Param("id")

	if err := h.store.SetDefaultDevice(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to set default device: " + err.Error(),
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    nil,
	})
}
