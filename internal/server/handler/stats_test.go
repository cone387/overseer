package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/overseer/overseer/internal/model"
	"github.com/overseer/overseer/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupStatsRouter(s store.Store) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewStatsHandler(s)
	api := r.Group("/api")
	h.RegisterRoutes(api)
	return r
}

func TestGetStats_Success(t *testing.T) {
	ms := new(mockStore)
	now := time.Now().UTC().Truncate(time.Second)
	from := now.Add(-24 * time.Hour)

	expected := []model.ChannelStats{
		{
			Channel:        "urgent",
			Total:          10,
			SuccessCount:   8,
			FailedCount:    2,
			SuccessPercent: 80.0,
			FailedPercent:  20.0,
		},
		{
			Channel:        "github",
			Total:          5,
			SuccessCount:   5,
			FailedCount:    0,
			SuccessPercent: 100.0,
			FailedPercent:  0.0,
		},
	}

	ms.On("GetChannelStats", mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time")).Return(expected, nil)

	router := setupStatsRouter(ms)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/stats?from="+from.Format(time.RFC3339)+"&to="+now.Format(time.RFC3339), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, float64(200), resp["code"])
	assert.Equal(t, "success", resp["message"])

	data := resp["data"].([]interface{})
	assert.Len(t, data, 2)

	first := data[0].(map[string]interface{})
	assert.Equal(t, "urgent", first["channel"])
	assert.Equal(t, float64(10), first["total"])
	assert.Equal(t, float64(8), first["success_count"])
	assert.Equal(t, float64(2), first["failed_count"])
	assert.Equal(t, float64(80.0), first["success_percent"])
	assert.Equal(t, float64(20.0), first["failed_percent"])

	ms.AssertExpectations(t)
}

func TestGetStats_MissingParams(t *testing.T) {
	ms := new(mockStore)
	router := setupStatsRouter(ms)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/stats", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, float64(400), resp["code"])
	assert.Contains(t, resp["message"], "required")
}

func TestGetStats_InvalidFromFormat(t *testing.T) {
	ms := new(mockStore)
	router := setupStatsRouter(ms)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/stats?from=not-a-date&to=2024-01-01T00:00:00Z", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Contains(t, resp["message"], "'from'")
}

func TestGetStats_InvalidToFormat(t *testing.T) {
	ms := new(mockStore)
	router := setupStatsRouter(ms)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/stats?from=2024-01-01T00:00:00Z&to=bad", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Contains(t, resp["message"], "'to'")
}

func TestGetStats_TimeRangeExceeds90Days(t *testing.T) {
	ms := new(mockStore)
	router := setupStatsRouter(ms)

	from := "2024-01-01T00:00:00Z"
	to := "2024-07-01T00:00:00Z" // > 90 days

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/stats?from="+from+"&to="+to, nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Contains(t, resp["message"], "90 days")
}

func TestGetStats_EmptyResult(t *testing.T) {
	ms := new(mockStore)
	now := time.Now().UTC().Truncate(time.Second)
	from := now.Add(-24 * time.Hour)

	ms.On("GetChannelStats", mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time")).Return([]model.ChannelStats{}, nil)

	router := setupStatsRouter(ms)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/stats?from="+from.Format(time.RFC3339)+"&to="+now.Format(time.RFC3339), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, float64(200), resp["code"])

	data := resp["data"].([]interface{})
	assert.Len(t, data, 0)

	ms.AssertExpectations(t)
}
