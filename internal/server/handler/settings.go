package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/overseer/overseer/internal/llm"
	"github.com/overseer/overseer/internal/store"
)

// SettingsHandler handles system settings API endpoints.
type SettingsHandler struct {
	store     store.Store
	llmClient **llm.Client // pointer to pointer so we can update the client at runtime
}

// NewSettingsHandler creates a new SettingsHandler.
func NewSettingsHandler(s store.Store, llmClient **llm.Client) *SettingsHandler {
	return &SettingsHandler{store: s, llmClient: llmClient}
}

// RegisterRoutes registers settings routes on the given router group.
func (h *SettingsHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/settings/llm", h.GetLLM)
	rg.POST("/settings/llm", h.SaveLLM)
	rg.GET("/settings/llm/models", h.ListModels)
}

type llmSettingsResponse struct {
	BaseURL    string `json:"base_url"`
	APIKey     string `json:"api_key"` // masked
	Model      string `json:"model"`
	Configured bool   `json:"configured"`
}

// GetLLM returns the current LLM configuration.
func (h *SettingsHandler) GetLLM(c *gin.Context) {
	baseURL, _ := h.store.GetSetting("llm.base_url")
	apiKey, _ := h.store.GetSetting("llm.api_key")
	model, _ := h.store.GetSetting("llm.model")

	masked := ""
	if apiKey != "" {
		if len(apiKey) > 8 {
			masked = apiKey[:4] + "****" + apiKey[len(apiKey)-4:]
		} else {
			masked = "****"
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": llmSettingsResponse{
			BaseURL:    baseURL,
			APIKey:     masked,
			Model:      model,
			Configured: apiKey != "",
		},
	})
}

type saveLLMRequest struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
	Model   string `json:"model"`
}

// SaveLLM saves LLM configuration and reinitializes the client.
func (h *SettingsHandler) SaveLLM(c *gin.Context) {
	var req saveLLMRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "invalid request",
			"data":    nil,
		})
		return
	}

	if req.BaseURL != "" {
		h.store.SetSetting("llm.base_url", req.BaseURL)
	}
	if req.APIKey != "" {
		h.store.SetSetting("llm.api_key", req.APIKey)
	}
	if req.Model != "" {
		h.store.SetSetting("llm.model", req.Model)
	}

	// Reinitialize the LLM client with new settings
	baseURL, _ := h.store.GetSetting("llm.base_url")
	apiKey, _ := h.store.GetSetting("llm.api_key")
	model, _ := h.store.GetSetting("llm.model")

	newClient := llm.NewClient(llm.Config{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Model:   model,
	})
	*h.llmClient = newClient

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "LLM 配置已保存",
		"data":    nil,
	})
}

// ListModels fetches available models from the configured LLM API.
func (h *SettingsHandler) ListModels(c *gin.Context) {
	if h.llmClient == nil || *h.llmClient == nil || !(*h.llmClient).IsConfigured() {
		c.JSON(http.StatusOK, gin.H{
			"code":    200,
			"message": "success",
			"data":    []string{},
		})
		return
	}

	models, err := (*h.llmClient).ListModels(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取模型列表失败: " + err.Error(),
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    models,
	})
}
