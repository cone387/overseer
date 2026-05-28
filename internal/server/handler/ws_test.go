package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/overseer/overseer/internal/server/middleware"
	"github.com/overseer/overseer/internal/ws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testWSSecret = "test-jwt-secret-for-ws-tests"

func generateWSTestToken() string {
	claims := middleware.JWTClaims{
		UserID:   "user-ws-test",
		Username: "wsuser",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString([]byte(testWSSecret))
	return tokenStr
}

func setupWSRouter() (*gin.Engine, *ws.Hub) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	hub := ws.NewHub(20)
	go hub.Run()

	h := NewWSHandler(hub, testWSSecret, nil)
	h.Register(engine)
	return engine, hub
}

func TestWSHandler_MissingToken(t *testing.T) {
	engine, _ := setupWSRouter()

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(401), resp["code"])
	assert.Contains(t, resp["message"], "unauthorized")
}

func TestWSHandler_InvalidToken(t *testing.T) {
	engine, _ := setupWSRouter()

	req := httptest.NewRequest(http.MethodGet, "/ws?token=wrong-token", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(401), resp["code"])
}

func TestWSHandler_ValidToken_UpgradesConnection(t *testing.T) {
	engine, hub := setupWSRouter()
	server := httptest.NewServer(engine)
	defer server.Close()

	validToken := generateWSTestToken()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?token=" + validToken

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	assert.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)

	// Give the hub time to register the client
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 1, hub.ClientCount())
}

func TestWSHandler_ValidToken_ReceivesBroadcast(t *testing.T) {
	engine, hub := setupWSRouter()
	server := httptest.NewServer(engine)
	defer server.Close()

	validToken := generateWSTestToken()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?token=" + validToken

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	// Wait for registration
	time.Sleep(50 * time.Millisecond)

	// Broadcast an event
	event := ws.Event{
		Type:    "push",
		Payload: map[string]string{"title": "test"},
	}
	hub.Broadcast(event)

	// Read the event from the WebSocket
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, message, err := conn.ReadMessage()
	require.NoError(t, err)

	var received ws.Event
	err = json.Unmarshal(message, &received)
	require.NoError(t, err)
	assert.Equal(t, "push", received.Type)
}

func TestWSHandler_ClientDisconnect_UnregistersFromHub(t *testing.T) {
	engine, hub := setupWSRouter()
	server := httptest.NewServer(engine)
	defer server.Close()

	validToken := generateWSTestToken()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?token=" + validToken

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)

	// Wait for registration
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 1, hub.ClientCount())

	// Close the connection
	conn.Close()

	// Wait for unregistration
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 0, hub.ClientCount())
}
