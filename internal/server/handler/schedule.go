package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/overseer/overseer/internal/llm"
)

// ScheduleHandler handles schedule parsing endpoints.
type ScheduleHandler struct {
	llmClient *llm.Client
}

// NewScheduleHandler creates a new ScheduleHandler.
func NewScheduleHandler(client *llm.Client) *ScheduleHandler {
	return &ScheduleHandler{llmClient: client}
}

// RegisterRoutes registers schedule routes on the given router group.
func (h *ScheduleHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/schedule/parse", h.Parse)
}

type parseRequest struct {
	Input    string `json:"input" binding:"required"`
	Timezone string `json:"timezone"`
}

// Parse handles POST /api/schedule/parse - parses natural language into a ScheduleConfig.
func (h *ScheduleHandler) Parse(c *gin.Context) {
	if h.llmClient == nil || !h.llmClient.IsConfigured() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "LLM 未配置，请在 config.yaml 中设置 llm.api_key",
			"data":    nil,
		})
		return
	}

	var req parseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "invalid request: input is required",
			"data":    nil,
		})
		return
	}

	result, err := llm.ParseSchedule(c.Request.Context(), h.llmClient, req.Input, req.Timezone)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "解析失败: " + err.Error(),
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    result,
	})
}
