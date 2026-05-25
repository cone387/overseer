package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/overseer/overseer/internal/model"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupPushRouter(handler MessageHandler, testHandler TestPushHandler) *gin.Engine {
	r := gin.New()
	ph := NewPushHandler(handler, testHandler)
	r.POST("/api/push", ph.HandlePush)
	r.POST("/api/push/test", ph.HandleTestPush)
	return r
}

func TestHandlePush_Success(t *testing.T) {
	var receivedMsg *model.Message
	handler := func(msg *model.Message) error {
		receivedMsg = msg
		return nil
	}
	testHandler := func(msg *model.Message) error { return nil }

	router := setupPushRouter(handler, testHandler)

	body := PushRequest{
		Title:   "Test Title",
		Body:    "Test Body",
		Channel: "urgent",
		Extra:   map[string]string{"key": "value"},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/push", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, float64(200), resp["code"])
	assert.Equal(t, "success", resp["message"])

	data := resp["data"].(map[string]interface{})
	assert.NotEmpty(t, data["id"])

	// Verify the message was constructed correctly
	assert.NotNil(t, receivedMsg)
	assert.Equal(t, "api", receivedMsg.Source)
	assert.Equal(t, "Test Title", receivedMsg.Title)
	assert.Equal(t, "Test Body", receivedMsg.Body)
	assert.Equal(t, "urgent", receivedMsg.Channel)
	assert.Equal(t, map[string]string{"key": "value"}, receivedMsg.Extra)
	assert.Equal(t, model.StatusPending, receivedMsg.Status)
	assert.NotEmpty(t, receivedMsg.ID)
}

func TestHandlePush_MissingTitle(t *testing.T) {
	handler := func(msg *model.Message) error { return nil }
	testHandler := func(msg *model.Message) error { return nil }

	router := setupPushRouter(handler, testHandler)

	body := PushRequest{
		Body: "Test Body",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/push", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, float64(400), resp["code"])
	assert.Contains(t, resp["message"], "title")
}

func TestHandlePush_MissingBody(t *testing.T) {
	handler := func(msg *model.Message) error { return nil }
	testHandler := func(msg *model.Message) error { return nil }

	router := setupPushRouter(handler, testHandler)

	body := PushRequest{
		Title: "Test Title",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/push", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, float64(400), resp["code"])
	assert.Contains(t, resp["message"], "body")
}

func TestHandlePush_InvalidJSON(t *testing.T) {
	handler := func(msg *model.Message) error { return nil }
	testHandler := func(msg *model.Message) error { return nil }

	router := setupPushRouter(handler, testHandler)

	req := httptest.NewRequest(http.MethodPost, "/api/push", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, float64(400), resp["code"])
	assert.Contains(t, resp["message"], "invalid JSON")
}

func TestHandlePush_HandlerError(t *testing.T) {
	handler := func(msg *model.Message) error {
		return errors.New("push pipeline error")
	}
	testHandler := func(msg *model.Message) error { return nil }

	router := setupPushRouter(handler, testHandler)

	body := PushRequest{
		Title: "Test Title",
		Body:  "Test Body",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/push", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, float64(500), resp["code"])
	assert.Contains(t, resp["message"], "push pipeline error")
}

func TestHandlePush_OptionalChannelDefaults(t *testing.T) {
	var receivedMsg *model.Message
	handler := func(msg *model.Message) error {
		receivedMsg = msg
		return nil
	}
	testHandler := func(msg *model.Message) error { return nil }

	router := setupPushRouter(handler, testHandler)

	body := PushRequest{
		Title: "Title",
		Body:  "Body",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/push", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotNil(t, receivedMsg)
	assert.Equal(t, "", receivedMsg.Channel) // empty channel, router will handle fallback
}

func TestHandleTestPush_Success(t *testing.T) {
	var receivedMsg *model.Message
	handler := func(msg *model.Message) error { return nil }
	testHandler := func(msg *model.Message) error {
		receivedMsg = msg
		return nil
	}

	router := setupPushRouter(handler, testHandler)

	body := PushRequest{
		Title:   "Test Push Title",
		Body:    "Test Push Body",
		Channel: "monitor",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/push/test", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, float64(200), resp["code"])
	assert.Equal(t, "success", resp["message"])

	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "delivered", data["result"])

	// Verify the message was constructed correctly
	assert.NotNil(t, receivedMsg)
	assert.Equal(t, "api-test", receivedMsg.Source)
	assert.Equal(t, "Test Push Title", receivedMsg.Title)
	assert.Equal(t, "Test Push Body", receivedMsg.Body)
	assert.Equal(t, "monitor", receivedMsg.Channel)
}

func TestHandleTestPush_MissingTitle(t *testing.T) {
	handler := func(msg *model.Message) error { return nil }
	testHandler := func(msg *model.Message) error { return nil }

	router := setupPushRouter(handler, testHandler)

	body := PushRequest{
		Body: "Test Body",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/push/test", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Contains(t, resp["message"], "title")
}

func TestHandleTestPush_MissingBody(t *testing.T) {
	handler := func(msg *model.Message) error { return nil }
	testHandler := func(msg *model.Message) error { return nil }

	router := setupPushRouter(handler, testHandler)

	body := PushRequest{
		Title: "Test Title",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/push/test", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Contains(t, resp["message"], "body")
}

func TestHandleTestPush_InvalidJSON(t *testing.T) {
	handler := func(msg *model.Message) error { return nil }
	testHandler := func(msg *model.Message) error { return nil }

	router := setupPushRouter(handler, testHandler)

	req := httptest.NewRequest(http.MethodPost, "/api/push/test", bytes.NewReader([]byte("{bad")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Contains(t, resp["message"], "invalid JSON")
}

func TestHandleTestPush_HandlerError(t *testing.T) {
	handler := func(msg *model.Message) error { return nil }
	testHandler := func(msg *model.Message) error {
		return errors.New("device unreachable")
	}

	router := setupPushRouter(handler, testHandler)

	body := PushRequest{
		Title: "Test Title",
		Body:  "Test Body",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/push/test", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, float64(500), resp["code"])
	assert.Contains(t, resp["message"], "device unreachable")
}

func TestHandleTestPush_DoesNotCallMainHandler(t *testing.T) {
	mainHandlerCalled := false
	handler := func(msg *model.Message) error {
		mainHandlerCalled = true
		return nil
	}
	testHandler := func(msg *model.Message) error { return nil }

	router := setupPushRouter(handler, testHandler)

	body := PushRequest{
		Title: "Test Title",
		Body:  "Test Body",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/push/test", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.False(t, mainHandlerCalled, "main handler should not be called for test push")
}

func TestHandlePush_WithExtra(t *testing.T) {
	var receivedMsg *model.Message
	handler := func(msg *model.Message) error {
		receivedMsg = msg
		return nil
	}
	testHandler := func(msg *model.Message) error { return nil }

	router := setupPushRouter(handler, testHandler)

	body := PushRequest{
		Title: "Alert",
		Body:  "Server down",
		Extra: map[string]string{
			"url":      "https://example.com",
			"severity": "high",
		},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/push", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotNil(t, receivedMsg)
	assert.Equal(t, "https://example.com", receivedMsg.Extra["url"])
	assert.Equal(t, "high", receivedMsg.Extra["severity"])
}
