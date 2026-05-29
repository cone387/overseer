package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/overseer/overseer/internal/repeatpusher"
	"github.com/overseer/overseer/internal/store"
	"github.com/overseer/overseer/internal/ws"
)

// SnoozeRequest is the request body for snoozing a message.
type SnoozeRequest struct {
	Duration string `json:"duration" binding:"required"`
}

// validSnoozeDurations defines the allowed snooze duration values.
var validSnoozeDurations = map[string]bool{
	"1h":           true,
	"2h":           true,
	"4h":           true,
	"tomorrow_9am": true,
}

// LifecycleHandler handles notification lifecycle API endpoints (ack, snooze).
type LifecycleHandler struct {
	store        store.Store
	hub          *ws.Hub
	repeatPusher *repeatpusher.RepeatPusher
}

// NewLifecycleHandler creates a new LifecycleHandler.
func NewLifecycleHandler(s store.Store, hub *ws.Hub, rp *repeatpusher.RepeatPusher) *LifecycleHandler {
	return &LifecycleHandler{store: s, hub: hub, repeatPusher: rp}
}

// RegisterRoutes registers lifecycle routes on the given router group.
func (h *LifecycleHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/messages/:id/ack", h.HandleAck)
	rg.POST("/messages/:id/snooze", h.HandleSnooze)
}

// HandleAck handles POST /api/messages/:id/ack.
// It records the acknowledgment timestamp for a message.
// Idempotent: if already acknowledged, returns 200 unchanged.
func (h *LifecycleHandler) HandleAck(c *gin.Context) {
	id := c.Param("id")

	msg, err := h.store.GetMessage(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to get message: " + err.Error(),
			"data":    nil,
		})
		return
	}
	if msg == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "message not found",
			"data":    nil,
		})
		return
	}

	// Idempotent: already acknowledged
	if msg.AckAt != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    200,
			"message": "ok",
			"data":    nil,
		})
		return
	}

	if err := h.store.AckMessage(id, time.Now()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to ack message: " + err.Error(),
			"data":    nil,
		})
		return
	}

	// Cancel repeat push for this message
	if h.repeatPusher != nil {
		h.repeatPusher.Cancel(id)
	}

	// Broadcast ack_sync event to all connected clients
	if h.hub != nil {
		h.hub.Broadcast(ws.Event{
			Type: "ack_sync",
			Payload: gin.H{
				"message_id": id,
			},
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "ok",
		"data":    nil,
	})
}

// HandleSnooze handles POST /api/messages/:id/snooze.
// It validates the snooze duration and records the snooze_until timestamp.
func (h *LifecycleHandler) HandleSnooze(c *gin.Context) {
	id := c.Param("id")

	var req SnoozeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "invalid request: " + err.Error(),
			"data":    nil,
		})
		return
	}

	if !validSnoozeDurations[req.Duration] {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "invalid duration: must be one of 1h, 2h, 4h, tomorrow_9am",
			"data":    nil,
		})
		return
	}

	msg, err := h.store.GetMessage(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to get message: " + err.Error(),
			"data":    nil,
		})
		return
	}
	if msg == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "message not found",
			"data":    nil,
		})
		return
	}

	snoozeUntil := computeSnoozeUntil(req.Duration, time.Now())

	if err := h.store.SnoozeMessage(id, snoozeUntil); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to snooze message: " + err.Error(),
			"data":    nil,
		})
		return
	}

	// Schedule snooze in repeat pusher
	if h.repeatPusher != nil {
		h.repeatPusher.ScheduleSnooze(id, snoozeUntil)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "ok",
		"data": gin.H{
			"snooze_until": snoozeUntil.Format(time.RFC3339),
		},
	})
}

// computeSnoozeUntil calculates the snooze_until time based on the duration string.
func computeSnoozeUntil(duration string, now time.Time) time.Time {
	switch duration {
	case "1h":
		return now.Add(1 * time.Hour)
	case "2h":
		return now.Add(2 * time.Hour)
	case "4h":
		return now.Add(4 * time.Hour)
	case "tomorrow_9am":
		tomorrow := now.AddDate(0, 0, 1)
		return time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 9, 0, 0, 0, now.Location())
	default:
		return now.Add(1 * time.Hour)
	}
}
