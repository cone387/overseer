package ws

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_IdleTimeout(t *testing.T) {
	// Use a short idle timeout for testing by creating a custom server.
	// We'll override pongWait by setting a short read deadline in the handler.
	shortTimeout := 2 * time.Second

	hub := NewHub(20)
	go hub.Run()

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		client := NewClient(hub, conn)
		hub.Register(client)

		// Override: use short timeout for test.
		go client.WritePump()
		go func() {
			defer func() {
				hub.Unregister(client)
				conn.Close()
			}()
			conn.SetReadLimit(maxMessageSize)
			conn.SetReadDeadline(time.Now().Add(shortTimeout))
			conn.SetPongHandler(func(string) error {
				conn.SetReadDeadline(time.Now().Add(shortTimeout))
				return nil
			})
			for {
				_, _, err := conn.ReadMessage()
				if err != nil {
					break
				}
				conn.SetReadDeadline(time.Now().Add(shortTimeout))
			}
		}()
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 1, hub.ClientCount())

	// Don't send any messages or pongs. Wait for the idle timeout.
	// The server will close the connection after shortTimeout.
	time.Sleep(shortTimeout + 500*time.Millisecond)

	assert.Equal(t, 0, hub.ClientCount())
}

func TestClient_MessageKeepsAlive(t *testing.T) {
	hub := NewHub(20)
	go hub.Run()

	server := newTestServer(t, hub)
	defer server.Close()

	conn := dialWS(t, server)
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 1, hub.ClientCount())

	// Send a message to keep the connection alive.
	err := conn.WriteMessage(websocket.TextMessage, []byte("hello"))
	require.NoError(t, err)

	// Connection should still be alive.
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 1, hub.ClientCount())
}

func TestClient_ReceivesEvents(t *testing.T) {
	hub := NewHub(20)
	go hub.Run()

	server := newTestServer(t, hub)
	defer server.Close()

	conn := dialWS(t, server)
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	// Broadcast an event.
	event := Event{
		Type:    "reminder_change",
		Payload: map[string]interface{}{"id": "123", "action": "created"},
	}
	hub.Broadcast(event)

	// Client should receive it.
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var received Event
	err := conn.ReadJSON(&received)
	require.NoError(t, err)
	assert.Equal(t, "reminder_change", received.Type)
}

func TestClient_MultipleClients_IndependentDisconnect(t *testing.T) {
	hub := NewHub(20)
	go hub.Run()

	server := newTestServer(t, hub)
	defer server.Close()

	conn1 := dialWS(t, server)
	conn2 := dialWS(t, server)
	defer conn2.Close()

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 2, hub.ClientCount())

	// Disconnect client 1.
	conn1.Close()
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 1, hub.ClientCount())

	// Client 2 should still receive broadcasts.
	hub.Broadcast(Event{Type: "push", Payload: "test"})
	conn2.SetReadDeadline(time.Now().Add(2 * time.Second))
	var received Event
	err := conn2.ReadJSON(&received)
	require.NoError(t, err)
	assert.Equal(t, "push", received.Type)
}

func TestNewClient(t *testing.T) {
	hub := NewHub(20)

	// Create a mock connection using a test server.
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	var serverConn *websocket.Conn
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		serverConn, err = upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	clientConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer clientConn.Close()

	time.Sleep(50 * time.Millisecond)
	require.NotNil(t, serverConn)

	client := NewClient(hub, serverConn)
	assert.NotNil(t, client)
	assert.Equal(t, hub, client.hub)
	assert.Equal(t, serverConn, client.conn)
	assert.NotNil(t, client.send)
}
