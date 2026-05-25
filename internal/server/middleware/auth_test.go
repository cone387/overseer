package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupRouter(apiKey string) *gin.Engine {
	r := gin.New()

	// Health endpoint - no auth
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok"})
	})

	// Protected routes
	protected := r.Group("/")
	protected.Use(APIKeyAuth(apiKey))
	protected.GET("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"code": 200, "message": "success"})
	})
	protected.POST("/webhook/github", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"code": 200, "message": "success"})
	})

	return r
}

func TestAPIKeyAuth_ValidKey(t *testing.T) {
	apiKey := "test-api-key-1234567890"
	router := setupRouter(apiKey)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("X-API-Key", apiKey)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, float64(200), resp["code"])
	assert.Equal(t, "success", resp["message"])
}

func TestAPIKeyAuth_MissingKey(t *testing.T) {
	apiKey := "test-api-key-1234567890"
	router := setupRouter(apiKey)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	// No X-API-Key header
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, float64(401), resp["code"])
	assert.Equal(t, "unauthorized: invalid or missing API key", resp["message"])
}

func TestAPIKeyAuth_InvalidKey(t *testing.T) {
	apiKey := "test-api-key-1234567890"
	router := setupRouter(apiKey)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("X-API-Key", "wrong-key-value")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, float64(401), resp["code"])
	assert.Equal(t, "unauthorized: invalid or missing API key", resp["message"])
}

func TestAPIKeyAuth_HealthEndpointNoAuth(t *testing.T) {
	apiKey := "test-api-key-1234567890"
	router := setupRouter(apiKey)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	// No X-API-Key header
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, float64(200), resp["code"])
	assert.Equal(t, "ok", resp["message"])
}

func TestAPIKeyAuth_WebhookRequiresAuth(t *testing.T) {
	apiKey := "test-api-key-1234567890"
	router := setupRouter(apiKey)

	// Without key
	req := httptest.NewRequest(http.MethodPost, "/webhook/github", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// With valid key
	req = httptest.NewRequest(http.MethodPost, "/webhook/github", nil)
	req.Header.Set("X-API-Key", apiKey)
	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAPIKeyAuth_EmptyKeyConfig(t *testing.T) {
	// When API key config is empty, all requests should be rejected
	router := setupRouter("")

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("X-API-Key", "")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
