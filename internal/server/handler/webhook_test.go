package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/overseer/overseer/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestHandleWebhook_Success(t *testing.T) {
	var captured *model.Message
	handler := func(msg *model.Message) error {
		captured = msg
		return nil
	}

	router := setupRouter(handler)

	body := WebhookRequest{
		Title:   "Test Title",
		Body:    "Test Body",
		Channel: "github",
		Extra:   map[string]string{"key": "value"},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/webhook/github", bytes.NewReader(jsonBody))
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

	// Verify captured message
	assert.NotNil(t, captured)
	assert.Equal(t, "github", captured.Source)
	assert.Equal(t, "Test Title", captured.Title)
	assert.Equal(t, "Test Body", captured.Body)
	assert.Equal(t, "github", captured.Channel)
	assert.Equal(t, map[string]string{"key": "value"}, captured.Extra)
	assert.Equal(t, model.StatusPending, captured.Status)
	assert.NotEmpty(t, captured.ID)
}

func TestHandleWebhook_SuccessMinimalFields(t *testing.T) {
	var captured *model.Message
	handler := func(msg *model.Message) error {
		captured = msg
		return nil
	}

	router := setupRouter(handler)

	body := `{"title": "Hello", "body": "World"}`
	req := httptest.NewRequest(http.MethodPost, "/webhook/grafana", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotNil(t, captured)
	assert.Equal(t, "grafana", captured.Source)
	assert.Equal(t, "Hello", captured.Title)
	assert.Equal(t, "World", captured.Body)
	assert.Empty(t, captured.Channel)
	assert.Nil(t, captured.Extra)
}

func TestHandleWebhook_InvalidJSON(t *testing.T) {
	handler := func(msg *model.Message) error { return nil }
	router := setupRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/webhook/test", strings.NewReader("not json"))
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

func TestHandleWebhook_MissingTitle(t *testing.T) {
	handler := func(msg *model.Message) error { return nil }
	router := setupRouter(handler)

	body := `{"body": "some body"}`
	req := httptest.NewRequest(http.MethodPost, "/webhook/test", strings.NewReader(body))
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

func TestHandleWebhook_MissingBody(t *testing.T) {
	handler := func(msg *model.Message) error { return nil }
	router := setupRouter(handler)

	body := `{"title": "some title"}`
	req := httptest.NewRequest(http.MethodPost, "/webhook/test", strings.NewReader(body))
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

func TestHandleWebhook_TitleTooLong(t *testing.T) {
	handler := func(msg *model.Message) error { return nil }
	router := setupRouter(handler)

	longTitle := strings.Repeat("a", 201)
	body := `{"title": "` + longTitle + `", "body": "test"}`
	req := httptest.NewRequest(http.MethodPost, "/webhook/test", strings.NewReader(body))
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

func TestHandleWebhook_BodyTooLong(t *testing.T) {
	handler := func(msg *model.Message) error { return nil }
	router := setupRouter(handler)

	longBody := strings.Repeat("b", 4001)
	reqBody := WebhookRequest{
		Title: "title",
		Body:  longBody,
	}
	jsonBody, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/webhook/test", bytes.NewReader(jsonBody))
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

func TestHandleWebhook_TitleExactly200Chars(t *testing.T) {
	var captured *model.Message
	handler := func(msg *model.Message) error {
		captured = msg
		return nil
	}
	router := setupRouter(handler)

	title := strings.Repeat("x", 200)
	reqBody := WebhookRequest{
		Title: title,
		Body:  "valid body",
	}
	jsonBody, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/webhook/test", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotNil(t, captured)
	assert.Equal(t, title, captured.Title)
}

func TestHandleWebhook_BodyExactly4000Chars(t *testing.T) {
	var captured *model.Message
	handler := func(msg *model.Message) error {
		captured = msg
		return nil
	}
	router := setupRouter(handler)

	body := strings.Repeat("y", 4000)
	reqBody := WebhookRequest{
		Title: "title",
		Body:  body,
	}
	jsonBody, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/webhook/test", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotNil(t, captured)
	assert.Equal(t, body, captured.Body)
}

func TestHandleWebhook_HandlerError(t *testing.T) {
	handler := func(msg *model.Message) error {
		return errors.New("router failed")
	}
	router := setupRouter(handler)

	body := `{"title": "test", "body": "test body"}`
	req := httptest.NewRequest(http.MethodPost, "/webhook/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, float64(500), resp["code"])
	assert.Contains(t, resp["message"], "failed to process message")
}

func TestHandleWebhook_SourceFromPath(t *testing.T) {
	var captured *model.Message
	handler := func(msg *model.Message) error {
		captured = msg
		return nil
	}
	router := setupRouter(handler)

	body := `{"title": "test", "body": "test body"}`
	req := httptest.NewRequest(http.MethodPost, "/webhook/my-custom-source", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotNil(t, captured)
	assert.Equal(t, "my-custom-source", captured.Source)
}

func TestHandleWebhook_EmptyRequestBody(t *testing.T) {
	handler := func(msg *model.Message) error { return nil }
	router := setupRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/webhook/test", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
