package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/overseer/overseer/internal/model"
	"github.com/overseer/overseer/internal/store"
)

// HistoryHandler handles message history queries.
type HistoryHandler struct {
	store store.Store
}

// NewHistoryHandler creates a new HistoryHandler.
func NewHistoryHandler(s store.Store) *HistoryHandler {
	return &HistoryHandler{store: s}
}

// RegisterRoutes registers the history handler routes on the given router group.
func (h *HistoryHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/messages", h.QueryMessages)
}

// QueryMessages handles GET /api/messages with filtering and pagination.
func (h *HistoryHandler) QueryMessages(c *gin.Context) {
	fromStr := c.Query("from")
	toStr := c.Query("to")

	if fromStr == "" || toStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "parameters 'from' and 'to' are required, expected RFC3339 format",
			"data":    nil,
		})
		return
	}

	fromTime, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "invalid 'from' parameter: expected RFC3339 format (e.g. 2024-01-01T00:00:00Z)",
			"data":    nil,
		})
		return
	}

	toTime, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "invalid 'to' parameter: expected RFC3339 format (e.g. 2024-01-01T00:00:00Z)",
			"data":    nil,
		})
		return
	}

	// Validate time range does not exceed 90 days
	if toTime.Sub(fromTime) > 90*24*time.Hour {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "time range exceeds maximum allowed span of 90 days",
			"data":    nil,
		})
		return
	}

	// Parse pagination
	page := 1
	if p := c.Query("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}

	pageSize := 20
	if ps := c.Query("page_size"); ps != "" {
		if v, err := strconv.Atoi(ps); err == nil && v > 0 {
			pageSize = v
		}
	}
	if pageSize > 100 {
		pageSize = 100
	}

	// Build filter
	filter := store.MessageFilter{
		From:     fromTime,
		To:       toTime,
		Page:     page,
		PageSize: pageSize,
	}

	if ch := c.Query("channel"); ch != "" {
		filter.Channel = ch
	}

	if st := c.Query("status"); st != "" {
		filter.Status = model.PushStatus(st)
	}

	result, err := h.store.QueryMessages(filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to query messages: " + err.Error(),
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"total":     result.Total,
			"page":      result.Page,
			"page_size": result.PageSize,
			"data":      result.Data,
		},
	})
}
