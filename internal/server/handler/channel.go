package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/overseer/overseer/internal/model"
	"github.com/overseer/overseer/internal/store"
)

// ChannelHandler handles channel management API endpoints.
type ChannelHandler struct {
	store store.Store
}

// NewChannelHandler creates a new ChannelHandler.
func NewChannelHandler(s store.Store) *ChannelHandler {
	return &ChannelHandler{store: s}
}

// RegisterRoutes registers channel routes on the given router group.
func (h *ChannelHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/channels", h.List)
	rg.POST("/channels", h.Create)
	rg.PUT("/channels/:id", h.Update)
	rg.DELETE("/channels/:id", h.Delete)
}

// CreateChannelRequest is the request body for creating a channel.
type CreateChannelRequest struct {
	Name       string   `json:"name" binding:"required,min=1,max=64"`
	Sound      string   `json:"sound"`
	Group      string   `json:"group"`
	Icon       string   `json:"icon"`
	Level      string   `json:"level"`
	DeviceKeys []string `json:"device_keys"`
}

// UpdateChannelRequest is the request body for updating a channel.
type UpdateChannelRequest struct {
	Name       *string  `json:"name" binding:"omitempty,min=1,max=64"`
	Sound      *string  `json:"sound"`
	Group      *string  `json:"group"`
	Icon       *string  `json:"icon"`
	Level      *string  `json:"level"`
	DeviceKeys []string `json:"device_keys"`
}

// List handles GET /api/channels.
func (h *ChannelHandler) List(c *gin.Context) {
	channels, err := h.store.ListChannels()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to list channels: " + err.Error(),
			"data":    nil,
		})
		return
	}

	if channels == nil {
		channels = []model.Channel{}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    channels,
	})
}

// Create handles POST /api/channels.
func (h *ChannelHandler) Create(c *gin.Context) {
	var req CreateChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "validation error: " + err.Error(),
			"data":    nil,
		})
		return
	}

	// Validate level
	if req.Level != "" {
		switch req.Level {
		case "active", "timeSensitive", "passive", "critical":
			// valid
		default:
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "validation error: level must be one of: active, timeSensitive, passive, critical",
				"data":    nil,
			})
			return
		}
	} else {
		req.Level = "active"
	}

	if req.DeviceKeys == nil {
		req.DeviceKeys = []string{}
	}

	now := time.Now()
	ch := &model.Channel{
		ID:         uuid.New().String(),
		Name:       req.Name,
		Sound:      req.Sound,
		Group:      req.Group,
		Icon:       req.Icon,
		Level:      req.Level,
		DeviceKeys: req.DeviceKeys,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := h.store.CreateChannel(ch); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to create channel: " + err.Error(),
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    ch,
	})
}

// Update handles PUT /api/channels/:id.
func (h *ChannelHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var req UpdateChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "validation error: " + err.Error(),
			"data":    nil,
		})
		return
	}

	// Fetch existing channels to find the one to update
	channels, err := h.store.ListChannels()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to fetch channels: " + err.Error(),
			"data":    nil,
		})
		return
	}

	var existing *model.Channel
	for i := range channels {
		if channels[i].ID == id {
			existing = &channels[i]
			break
		}
	}

	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "channel not found",
			"data":    nil,
		})
		return
	}

	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Sound != nil {
		existing.Sound = *req.Sound
	}
	if req.Group != nil {
		existing.Group = *req.Group
	}
	if req.Icon != nil {
		existing.Icon = *req.Icon
	}
	if req.Level != nil {
		existing.Level = *req.Level
	}
	if req.DeviceKeys != nil {
		existing.DeviceKeys = req.DeviceKeys
	}

	if err := h.store.UpdateChannel(existing); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to update channel: " + err.Error(),
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

// Delete handles DELETE /api/channels/:id.
func (h *ChannelHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	// Prevent deleting the default channel
	ch, err := h.store.GetChannelByName("default")
	if err == nil && ch != nil && ch.ID == id {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "默认频道不能删除",
			"data":    nil,
		})
		return
	}

	if err := h.store.DeleteChannel(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to delete channel: " + err.Error(),
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
