package handler

import (
	"bytes"
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
	"github.com/stretchr/testify/require"
)

func setupReminderRouter(s store.Store, scheduleFn ScheduleFunc, cancelFn CancelFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewReminderHandler(s, scheduleFn, cancelFn)
	api := r.Group("/api")
	h.RegisterRoutes(api)
	return r
}

func TestCreateReminder_Success(t *testing.T) {
	ms := new(mockStore)
	ms.On("CreateReminder", mock.AnythingOfType("*model.Reminder")).Return(nil)

	router := setupReminderRouter(ms, nil, nil)

	triggerAt := time.Now().Add(1 * time.Hour).Format(time.RFC3339)
	body := map[string]interface{}{
		"title":      "Test Reminder",
		"body":       "This is a test",
		"trigger_at": triggerAt,
		"channel":    "urgent",
		"repeat":     "once",
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/reminders", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(200), resp["code"])
	assert.Equal(t, "success", resp["message"])

	data := resp["data"].(map[string]interface{})
	assert.NotEmpty(t, data["id"])

	ms.AssertExpectations(t)
}

func TestCreateReminder_DefaultChannel(t *testing.T) {
	ms := new(mockStore)
	ms.On("CreateReminder", mock.MatchedBy(func(r *model.Reminder) bool {
		return r.Channel == "default"
	})).Return(nil)

	router := setupReminderRouter(ms, nil, nil)

	triggerAt := time.Now().Add(1 * time.Hour).Format(time.RFC3339)
	body := map[string]interface{}{
		"title":      "No Channel",
		"trigger_at": triggerAt,
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/reminders", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	ms.AssertExpectations(t)
}

func TestCreateReminder_TriggerAtInPast(t *testing.T) {
	ms := new(mockStore)
	router := setupReminderRouter(ms, nil, nil)

	triggerAt := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	body := map[string]interface{}{
		"title":      "Past Reminder",
		"trigger_at": triggerAt,
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/reminders", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Contains(t, resp["message"], "must not be in the past")
}

func TestCreateReminder_TriggerAtInPast_RepeatingAllowed(t *testing.T) {
	ms := new(mockStore)
	ms.On("CreateReminder", mock.AnythingOfType("*model.Reminder")).Return(nil)

	router := setupReminderRouter(ms, nil, nil)

	triggerAt := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	body := map[string]interface{}{
		"title":      "Daily Reminder",
		"trigger_at": triggerAt,
		"repeat":     "daily",
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/reminders", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	ms.AssertExpectations(t)
}

func TestCreateReminder_InvalidTriggerAt(t *testing.T) {
	ms := new(mockStore)
	router := setupReminderRouter(ms, nil, nil)

	body := map[string]interface{}{
		"title":      "Bad Time",
		"trigger_at": "not-a-time",
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/reminders", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Contains(t, resp["message"], "RFC3339")
}

func TestCreateReminder_MissingTitle(t *testing.T) {
	ms := new(mockStore)
	router := setupReminderRouter(ms, nil, nil)

	triggerAt := time.Now().Add(1 * time.Hour).Format(time.RFC3339)
	body := map[string]interface{}{
		"trigger_at": triggerAt,
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/reminders", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateReminder_InvalidRepeat(t *testing.T) {
	ms := new(mockStore)
	router := setupReminderRouter(ms, nil, nil)

	triggerAt := time.Now().Add(1 * time.Hour).Format(time.RFC3339)
	body := map[string]interface{}{
		"title":      "Bad Repeat",
		"trigger_at": triggerAt,
		"repeat":     "invalid",
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/reminders", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Contains(t, resp["message"], "repeat must be one of")
}

func TestCreateReminder_WithScheduleCallback(t *testing.T) {
	ms := new(mockStore)
	ms.On("CreateReminder", mock.AnythingOfType("*model.Reminder")).Return(nil)

	var scheduled *model.Reminder
	scheduleFn := func(r *model.Reminder) error {
		scheduled = r
		return nil
	}

	router := setupReminderRouter(ms, scheduleFn, nil)

	triggerAt := time.Now().Add(1 * time.Hour).Format(time.RFC3339)
	body := map[string]interface{}{
		"title":      "Scheduled",
		"trigger_at": triggerAt,
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/reminders", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotNil(t, scheduled)
	assert.Equal(t, "Scheduled", scheduled.Title)
	ms.AssertExpectations(t)
}

func TestListReminders_All(t *testing.T) {
	ms := new(mockStore)
	now := time.Now()
	reminders := []model.Reminder{
		{ID: "1", Title: "R1", Status: "active", CreatedAt: now, UpdatedAt: now},
		{ID: "2", Title: "R2", Status: "cancelled", CreatedAt: now, UpdatedAt: now},
	}
	ms.On("ListReminders", store.ReminderFilter{Status: ""}).Return(reminders, nil)

	router := setupReminderRouter(ms, nil, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/reminders", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].([]interface{})
	assert.Len(t, data, 2)
	ms.AssertExpectations(t)
}

func TestListReminders_FilterByStatus(t *testing.T) {
	ms := new(mockStore)
	now := time.Now()
	reminders := []model.Reminder{
		{ID: "1", Title: "R1", Status: "active", CreatedAt: now, UpdatedAt: now},
		{ID: "3", Title: "R3", Status: "active", CreatedAt: now, UpdatedAt: now},
	}
	ms.On("ListReminders", store.ReminderFilter{Status: "active"}).Return(reminders, nil)

	router := setupReminderRouter(ms, nil, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/reminders?status=active", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].([]interface{})
	assert.Len(t, data, 2)
	ms.AssertExpectations(t)
}

func TestListReminders_InvalidStatus(t *testing.T) {
	ms := new(mockStore)
	router := setupReminderRouter(ms, nil, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/reminders?status=invalid", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateReminder_Success(t *testing.T) {
	ms := new(mockStore)
	now := time.Now()
	triggerAt := now.Add(2 * time.Hour)
	existing := []model.Reminder{
		{
			ID:          "abc-123",
			Title:       "Original",
			Body:        "Original body",
			Channel:     "default",
			TriggerAt:   triggerAt,
			RepeatType:  model.RepeatOnce,
			Status:      "active",
			NextTrigger: &triggerAt,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
	ms.On("ListReminders", store.ReminderFilter{}).Return(existing, nil)
	ms.On("UpdateReminder", mock.AnythingOfType("*model.Reminder")).Return(nil)

	router := setupReminderRouter(ms, nil, nil)

	body := map[string]interface{}{
		"title": "Updated Title",
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/reminders/abc-123", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, float64(200), resp["code"])
	ms.AssertExpectations(t)
}

func TestUpdateReminder_NotFound(t *testing.T) {
	ms := new(mockStore)
	ms.On("ListReminders", store.ReminderFilter{}).Return([]model.Reminder{}, nil)

	router := setupReminderRouter(ms, nil, nil)

	body := map[string]interface{}{
		"title": "Updated",
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/reminders/nonexistent", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	ms.AssertExpectations(t)
}

func TestUpdateReminder_InvalidTriggerAtInPast(t *testing.T) {
	ms := new(mockStore)
	now := time.Now()
	triggerAt := now.Add(2 * time.Hour)
	existing := []model.Reminder{
		{
			ID:          "abc-123",
			Title:       "Original",
			TriggerAt:   triggerAt,
			RepeatType:  model.RepeatOnce,
			Status:      "active",
			NextTrigger: &triggerAt,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
	ms.On("ListReminders", store.ReminderFilter{}).Return(existing, nil)

	router := setupReminderRouter(ms, nil, nil)

	pastTime := now.Add(-1 * time.Hour).Format(time.RFC3339)
	body := map[string]interface{}{
		"trigger_at": pastTime,
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/reminders/abc-123", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Contains(t, resp["message"], "must not be in the past")
	ms.AssertExpectations(t)
}

func TestDeleteReminder_Success(t *testing.T) {
	ms := new(mockStore)
	ms.On("CancelReminder", "abc-123").Return(nil)

	var cancelledID string
	cancelFn := func(id string) error {
		cancelledID = id
		return nil
	}

	router := setupReminderRouter(ms, nil, cancelFn)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/reminders/abc-123", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, float64(200), resp["code"])
	assert.Equal(t, "abc-123", cancelledID)
	ms.AssertExpectations(t)
}

func TestCreateReminder_InvalidJSON(t *testing.T) {
	ms := new(mockStore)
	router := setupReminderRouter(ms, nil, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/reminders", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
