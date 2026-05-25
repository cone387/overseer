package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthHandler handles the health check endpoint.
type HealthHandler struct{}

// NewHealthHandler creates a new HealthHandler.
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// Register registers the health check route on the given engine.
// The health endpoint is registered without authentication middleware.
func (h *HealthHandler) Register(engine *gin.Engine) {
	engine.GET("/health", h.Health)
}

// Health returns the service health status.
func (h *HealthHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "ok",
		"data": gin.H{
			"status": "healthy",
		},
	})
}
