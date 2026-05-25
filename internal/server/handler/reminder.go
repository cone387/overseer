package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/overseer/overseer/internal/model"
	"github.com/overseer/overseer/internal/store"
)

// ScheduleFunc is called when a reminder is created or updated to schedule it.
type ScheduleFunc func(r *model.Reminder) error

// CancelFunc is called when a reminder is cancelled to remove its schedule.
type CancelFunc func(id string) error

// ReminderHandler handles reminder CRUD API endpoints.
type ReminderHandler struct {
	store      store.Store
	scheduleFn ScheduleFunc
	cancelFn   CancelFunc
}

// NewReminderHandler creates a new ReminderHandler.
func NewReminderHandler(s store.Store, scheduleFn ScheduleFunc, cancelFn CancelFunc) *ReminderHandler {
	return &ReminderHandler{
		store:      s,
		scheduleFn: scheduleFn,
		cancelFn:   cancelFn,
	}
}

// RegisterRoutes registers reminder routes on the given router group.
func (h *ReminderHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/reminders", h.Create)
	rg.GET("/reminders", h.List)
	rg.PUT("/reminders/:id", h.Update)
	rg.DELETE("/reminders/:id", h.Delete)
}

// CreateReminderRequest is the request body for creating a reminder.
type CreateReminderRequest struct {
	Title      string `json:"title" binding:"required,min=1,max=200"`
	Body       string `json:"body" binding:"max=4000"`
	TriggerAt  string `json:"trigger_at" binding:"required"`
	Channel    string `json:"channel"`
	Repeat     string `json:"repeat"`
	RepeatRule string `json:"repeat_rule"`
}

// UpdateReminderRequest is the request body for updating a reminder.
type UpdateReminderRequest struct {
	Title      *string `json:"title" binding:"omitempty,min=1,max=200"`
	Body       *string `json:"body" binding:"omitempty,max=4000"`
	TriggerAt  *string `json:"trigger_at"`
	Channel    *string `json:"channel"`
	Repeat     *string `json:"repeat"`
	RepeatRule *string `json:"repeat_rule"`
}

// Create handles POST /api/reminders.
func (h *ReminderHandler) Create(c *gin.Context) {
	var req CreateReminderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "validation error: " + err.Error(),
			"data":    nil,
		})
		return
	}

	triggerAt, err := time.Parse(time.RFC3339, req.TriggerAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "validation error: trigger_at must be a valid RFC3339 timestamp",
			"data":    nil,
		})
		return
	}

	// Determine repeat type, default to "once"
	repeatType := model.RepeatOnce
	if req.Repeat != "" {
		switch model.RepeatType(req.Repeat) {
		case model.RepeatOnce, model.RepeatDaily, model.RepeatWeekly, model.RepeatCron:
			repeatType = model.RepeatType(req.Repeat)
		default:
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "validation error: repeat must be one of: once, daily, weekly, cron",
				"data":    nil,
			})
			return
		}
	}

	// Validate trigger_at is not in the past for once reminders
	if repeatType == model.RepeatOnce && triggerAt.Before(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "validation error: trigger_at must not be in the past for one-time reminders",
			"data":    nil,
		})
		return
	}

	channel := req.Channel
	if channel == "" {
		channel = "default"
	}

	now := time.Now()
	reminder := &model.Reminder{
		ID:         uuid.New().String(),
		Title:      req.Title,
		Body:       req.Body,
		Channel:    channel,
		TriggerAt:  triggerAt,
		RepeatType: repeatType,
		RepeatRule: req.RepeatRule,
		Status:     "active",
		NextTrigger: &triggerAt,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := h.store.CreateReminder(reminder); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to create reminder: " + err.Error(),
			"data":    nil,
		})
		return
	}

	if h.scheduleFn != nil {
		if err := h.scheduleFn(reminder); err != nil {
			// Log but don't fail the request - reminder is persisted
			_ = err
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"id": reminder.ID,
		},
	})
}

// List handles GET /api/reminders.
func (h *ReminderHandler) List(c *gin.Context) {
	status := c.Query("status")

	// Validate status if provided
	if status != "" {
		switch status {
		case "active", "completed", "cancelled":
			// valid
		default:
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "validation error: status must be one of: active, completed, cancelled",
				"data":    nil,
			})
			return
		}
	}

	filter := store.ReminderFilter{
		Status: status,
	}

	reminders, err := h.store.ListReminders(filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to list reminders: " + err.Error(),
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    reminders,
	})
}

// Update handles PUT /api/reminders/:id.
func (h *ReminderHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var req UpdateReminderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "validation error: " + err.Error(),
			"data":    nil,
		})
		return
	}

	// Fetch existing reminder
	filter := store.ReminderFilter{}
	reminders, err := h.store.ListReminders(filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to fetch reminder: " + err.Error(),
			"data":    nil,
		})
		return
	}

	var existing *model.Reminder
	for i := range reminders {
		if reminders[i].ID == id {
			existing = &reminders[i]
			break
		}
	}

	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "reminder not found",
			"data":    nil,
		})
		return
	}

	// Apply updates
	if req.Title != nil {
		existing.Title = *req.Title
	}
	if req.Body != nil {
		existing.Body = *req.Body
	}
	if req.Channel != nil {
		existing.Channel = *req.Channel
	}
	if req.Repeat != nil {
		switch model.RepeatType(*req.Repeat) {
		case model.RepeatOnce, model.RepeatDaily, model.RepeatWeekly, model.RepeatCron:
			existing.RepeatType = model.RepeatType(*req.Repeat)
		default:
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "validation error: repeat must be one of: once, daily, weekly, cron",
				"data":    nil,
			})
			return
		}
	}
	if req.RepeatRule != nil {
		existing.RepeatRule = *req.RepeatRule
	}
	if req.TriggerAt != nil {
		triggerAt, err := time.Parse(time.RFC3339, *req.TriggerAt)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "validation error: trigger_at must be a valid RFC3339 timestamp",
				"data":    nil,
			})
			return
		}
		// Validate trigger_at is not in the past for once reminders
		if existing.RepeatType == model.RepeatOnce && triggerAt.Before(time.Now()) {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "validation error: trigger_at must not be in the past for one-time reminders",
				"data":    nil,
			})
			return
		}
		existing.TriggerAt = triggerAt
		existing.NextTrigger = &triggerAt
	}

	existing.UpdatedAt = time.Now()

	if err := h.store.UpdateReminder(existing); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to update reminder: " + err.Error(),
			"data":    nil,
		})
		return
	}

	if h.scheduleFn != nil {
		_ = h.scheduleFn(existing)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    nil,
	})
}

// Delete handles DELETE /api/reminders/:id.
func (h *ReminderHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	if err := h.store.CancelReminder(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to cancel reminder: " + err.Error(),
			"data":    nil,
		})
		return
	}

	if h.cancelFn != nil {
		_ = h.cancelFn(id)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    nil,
	})
}
