package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/overseer/overseer/internal/config"
	"github.com/overseer/overseer/internal/server/middleware"
)

// Server wraps the Gin engine and HTTP server for Overseer.
type Server struct {
	cfg    *config.Config
	engine *gin.Engine
	srv    *http.Server
}

// NewServer creates a new Server with the given configuration.
func NewServer(cfg *config.Config) *Server {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())

	s := &Server{
		cfg:    cfg,
		engine: engine,
	}
	return s
}

// SetupRoutes registers all route groups and middleware.
func (s *Server) SetupRoutes() {
	// Health check endpoint - no auth required
	s.engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"code":    200,
			"message": "ok",
			"data": gin.H{
				"status": "healthy",
			},
		})
	})

	// Apply auth middleware to all other routes
	authGroup := s.engine.Group("/")
	authGroup.Use(middleware.APIKeyAuth(s.cfg.Server.APIKey))

	// API routes will be registered here by handlers
	api := authGroup.Group("/api")
	_ = api // placeholder for future handler registration
}

// Start begins listening on the configured port.
func (s *Server) Start() error {
	s.srv = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.cfg.Server.Port),
		Handler: s.engine,
	}
	return s.srv.ListenAndServe()
}

// Shutdown gracefully shuts down the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.srv == nil {
		return nil
	}
	return s.srv.Shutdown(ctx)
}

// Engine returns the underlying Gin engine (useful for testing).
func (s *Server) Engine() *gin.Engine {
	return s.engine
}
