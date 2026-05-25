package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupHealthRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	h := NewHealthHandler()
	h.Register(engine)
	return engine
}

func TestHealth_ReturnsOK(t *testing.T) {
	router := setupHealthRouter()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, float64(200), resp["code"])
	assert.Equal(t, "ok", resp["message"])

	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "healthy", data["status"])
}

func TestHealth_NoAuthRequired(t *testing.T) {
	// Health endpoint should work without any API key header
	router := setupHealthRouter()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHealth_MethodNotAllowed(t *testing.T) {
	router := setupHealthRouter()

	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
