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

func setupHistoryRouter(s store.Store) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHistoryHandler(s)
	api := r.Group("/api")
	h.RegisterRoutes(api)
	return r
}

func TestQueryMessages_Success(t *testing.T) {
	ms := new(mockStore)
	now := time.Now().UTC().Truncate(time.Second)
	from := now.Add(-24 * time.Hour)

	expected := &model.PagedResult[model.Message]{
		Total:    1,
		Page:     1,
		PageSize: 20,
		Data: []model.Message{
			{
				ID:         "msg-1",
				Source:     "github",
				Channel:    "dev",
				Title:      "Test",
				Body:       "Body",
				Status:     model.StatusSuccess,
				ReceivedAt: now,
			},
		},
	}

	ms.On("QueryMessages", mock.MatchedBy(func(f store.MessageFilter) bool {
		return f.Page == 1 && f.PageSize == 20
	})).Return(expected, nil)

	router := setupHistoryRouter(ms)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/messages?from="+from.Format(time.RFC3339)+"&to="+now.Format(time.RFC3339), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, float64(200), resp["code"])
	assert.Equal(t, "success", resp["message"])

	data := resp["data"].(map[string]interface{})
	assert.Equal(t, float64(1), data["total"])
	assert.Equal(t, float64(1), data["page"])
	assert.Equal(t, float64(20), data["page_size"])
	ms.AssertExpectations(t)
}

func TestQueryMessages_MissingParams(t *testing.T) {
	ms := new(mockStore)
	router := setupHistoryRouter(ms)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/messages", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, float64(400), resp["code"])
	assert.Contains(t, resp["message"], "required")
}

func TestQueryMessages_InvalidFromFormat(t *testing.T) {
	ms := new(mockStore)
	router := setupHistoryRouter(ms)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/messages?from=invalid&to=2024-01-01T00:00:00Z", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Contains(t, resp["message"], "'from'")
}

func TestQueryMessages_InvalidToFormat(t *testing.T) {
	ms := new(mockStore)
	router := setupHistoryRouter(ms)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/messages?from=2024-01-01T00:00:00Z&to=bad-date", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Contains(t, resp["message"], "'to'")
}

func TestQueryMessages_TimeRangeExceeds90Days(t *testing.T) {
	ms := new(mockStore)
	router := setupHistoryRouter(ms)

	from := "2024-01-01T00:00:00Z"
	to := "2024-06-01T00:00:00Z" // > 90 days

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/messages?from="+from+"&to="+to, nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Contains(t, resp["message"], "90 days")
}

func TestQueryMessages_PageSizeCappedAt100(t *testing.T) {
	ms := new(mockStore)
	now := time.Now().UTC().Truncate(time.Second)
	from := now.Add(-24 * time.Hour)

	ms.On("QueryMessages", mock.MatchedBy(func(f store.MessageFilter) bool {
		return f.PageSize == 100
	})).Return(&model.PagedResult[model.Message]{
		Total:    0,
		Page:     1,
		PageSize: 100,
		Data:     []model.Message{},
	}, nil)

	router := setupHistoryRouter(ms)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/messages?from="+from.Format(time.RFC3339)+"&to="+now.Format(time.RFC3339)+"&page_size=200", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	ms.AssertExpectations(t)
}

func TestQueryMessages_WithChannelAndStatusFilter(t *testing.T) {
	ms := new(mockStore)
	now := time.Now().UTC().Truncate(time.Second)
	from := now.Add(-24 * time.Hour)

	ms.On("QueryMessages", mock.MatchedBy(func(f store.MessageFilter) bool {
		return f.Channel == "urgent" && f.Status == model.StatusFailed
	})).Return(&model.PagedResult[model.Message]{
		Total:    0,
		Page:     1,
		PageSize: 20,
		Data:     []model.Message{},
	}, nil)

	router := setupHistoryRouter(ms)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/messages?from="+from.Format(time.RFC3339)+"&to="+now.Format(time.RFC3339)+"&channel=urgent&status=failed", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	ms.AssertExpectations(t)
}
