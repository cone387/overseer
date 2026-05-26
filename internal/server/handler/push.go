package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/overseer/overseer/internal/model"
)

// TestPushHandler is a callback that processes a test push message.
// Unlike MessageHandler, it does not record the message to history.
type TestPushHandler func(msg *model.Message) error

// PushHandler handles push API requests for sending instant notifications.
type PushHandler struct {
	handler     MessageHandler
	testHandler TestPushHandler
}

// PushRequest represents the expected JSON body for push API requests.
type PushRequest struct {
	Title   string            `json:"title"`
	Body    string            `json:"body"`
	Channel string            `json:"channel"`
	Sound   string            `json:"sound"`
	Icon    string            `json:"icon"`
	Group   string            `json:"group"`
	Level   string            `json:"level"`
	URL     string            `json:"url"`
	Extra   map[string]string `json:"extra"`
}

// NewPushHandler creates a new PushHandler with the given message handler
// and test push handler callbacks.
func NewPushHandler(handler MessageHandler, testHandler TestPushHandler) *PushHandler {
	return &PushHandler{
		handler:     handler,
		testHandler: testHandler,
	}
}

// HandlePush is a gin.HandlerFunc that processes POST /api/push requests.
// It validates the request, constructs a message, and processes it through the pipeline.
// The message is recorded to push history.
func (h *PushHandler) HandlePush(c *gin.Context) {
	var req PushRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "invalid JSON: " + err.Error(),
			"data":    nil,
		})
		return
	}

	if err := validatePushRequest(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
			"data":    nil,
		})
		return
	}

	msg := &model.Message{
		ID:         uuid.New().String(),
		Source:     "api",
		Channel:    req.Channel,
		Title:      req.Title,
		Body:       req.Body,
		Extra:      req.Extra,
		Sound:      req.Sound,
		Icon:       req.Icon,
		Group:      req.Group,
		Level:      req.Level,
		URL:        req.URL,
		Status:     model.StatusPending,
		ReceivedAt: time.Now(),
	}

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

// HandleTestPush is a gin.HandlerFunc that processes POST /api/push/test requests.
// It validates the request and sends a test push without recording to history.
func (h *PushHandler) HandleTestPush(c *gin.Context) {
	var req PushRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "invalid JSON: " + err.Error(),
			"data":    nil,
		})
		return
	}

	if err := validatePushRequest(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
			"data":    nil,
		})
		return
	}

	msg := &model.Message{
		ID:         uuid.New().String(),
		Source:     "api-test",
		Channel:    req.Channel,
		Title:      req.Title,
		Body:       req.Body,
		Extra:      req.Extra,
		Sound:      req.Sound,
		Icon:       req.Icon,
		Group:      req.Group,
		Level:      req.Level,
		URL:        req.URL,
		Status:     model.StatusPending,
		ReceivedAt: time.Now(),
	}

	if err := h.testHandler(msg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "test push failed: " + err.Error(),
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"result": "delivered",
		},
	})
}

// validatePushRequest validates the push request fields.
func validatePushRequest(req *PushRequest) error {
	if req.Title == "" {
		return &validationError{field: "title", reason: "is required"}
	}
	if req.Body == "" {
		return &validationError{field: "body", reason: "is required"}
	}
	return nil
}

// validationError represents a field validation error.
type validationError struct {
	field  string
	reason string
}

func (e *validationError) Error() string {
	return "validation error: field '" + e.field + "' " + e.reason
}
