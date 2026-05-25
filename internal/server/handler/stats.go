package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/overseer/overseer/internal/store"
)

// StatsHandler handles push statistics queries.
type StatsHandler struct {
	store store.Store
}

// NewStatsHandler creates a new StatsHandler.
func NewStatsHandler(s store.Store) *StatsHandler {
	return &StatsHandler{store: s}
}

// RegisterRoutes registers the stats handler routes on the given router group.
func (h *StatsHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/stats", h.GetStats)
}

// GetStats handles GET /api/stats with time range filtering.
func (h *StatsHandler) GetStats(c *gin.Context) {
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

	stats, err := h.store.GetChannelStats(fromTime, toTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to get channel stats: " + err.Error(),
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    stats,
	})
}
