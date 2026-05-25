package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/overseer/overseer/internal/model"
)

// MessageHandler is a callback function that processes a message after it has been
// validated and constructed by the webhook handler.
type MessageHandler func(msg *model.Message) error

// WebhookHandler handles incoming webhook requests from external systems.
type WebhookHandler struct {
	handler MessageHandler
}

// WebhookRequest represents the expected JSON body for webhook requests.
type WebhookRequest struct {
	Title   string            `json:"title"`
	Body    string            `json:"body"`
	Channel string            `json:"channel"`
	Extra   map[string]string `json:"extra"`
}

// NewWebhookHandler creates a new WebhookHandler with the given message handler callback.
func NewWebhookHandler(handler MessageHandler) *WebhookHandler {
	return &WebhookHandler{handler: handler}
}

// HandleWebhook is a gin.HandlerFunc that processes POST /webhook/{source} requests.
func (h *WebhookHandler) HandleWebhook(c *gin.Context) {
	source := c.Param("source")
	if source == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "missing source path parameter",
			"data":    nil,
		})
		return
	}

	var req WebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "invalid JSON: " + err.Error(),
			"data":    nil,
		})
		return
	}

	// Validate title: required, 1-200 characters
	if req.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "validation error: field 'title' is required",
			"data":    nil,
		})
		return
	}
	if len([]rune(req.Title)) > 200 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "validation error: field 'title' must be between 1 and 200 characters",
			"data":    nil,
		})
		return
	}

	// Validate body: required, 1-4000 characters
	if req.Body == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "validation error: field 'body' is required",
			"data":    nil,
		})
		return
	}
	if len([]rune(req.Body)) > 4000 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "validation error: field 'body' must be between 1 and 4000 characters",
			"data":    nil,
		})
		return
	}

	// Construct internal message
	msg := &model.Message{
		ID:         uuid.New().String(),
		Source:     source,
		Channel:    req.Channel,
		Title:      req.Title,
		Body:       req.Body,
		Extra:      req.Extra,
		Status:     model.StatusPending,
		ReceivedAt: time.Now(),
	}

	// Call the message handler
	if err := h.handler(msg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to process message: " + err.Error(),
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"id": msg.ID,
		},
	})
}
