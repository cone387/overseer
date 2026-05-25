package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/overseer/overseer/internal/config"
	"github.com/stretchr/testify/assert"
)

func testConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Port:   8080,
			APIKey: "test-api-key-1234567890",
		},
	}
}

func TestNewServer(t *testing.T) {
	cfg := testConfig()
	s := NewServer(cfg)

	assert.NotNil(t, s)
	assert.NotNil(t, s.engine)
	assert.Equal(t, cfg, s.cfg)
}

func TestServer_HealthEndpoint(t *testing.T) {
	cfg := testConfig()
	s := NewServer(cfg)
	s.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	s.engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, float64(200), resp["code"])
	assert.Equal(t, "ok", resp["message"])
}

func TestServer_HealthEndpointNoAuthRequired(t *testing.T) {
	cfg := testConfig()
	s := NewServer(cfg)
	s.SetupRoutes()

	// No API key header - should still work for /health
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	s.engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestServer_Shutdown(t *testing.T) {
	cfg := testConfig()
	s := NewServer(cfg)
	s.SetupRoutes()

	// Shutdown without Start should not error
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := s.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestServer_Engine(t *testing.T) {
	cfg := testConfig()
	s := NewServer(cfg)

	assert.NotNil(t, s.Engine())
	assert.Equal(t, s.engine, s.Engine())
}
