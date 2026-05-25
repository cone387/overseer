package ws

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestServer creates an httptest server that upgrades connections to WebSocket
// and registers them with the given hub.
func newTestServer(t *testing.T, hub *Hub) *httptest.Server {
	t.Helper()
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("upgrade error: %v", err)
			return
		}
		client := NewClient(hub, conn)
		hub.Register(client)
		go client.WritePump()
		go client.ReadPump()
	}))
	return server
}

// dialWS connects to the test server's WebSocket endpoint.
func dialWS(t *testing.T, server *httptest.Server) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	return conn
}

func TestNewHub_DefaultMaxConns(t *testing.T) {
	hub := NewHub(0)
	assert.Equal(t, DefaultMaxConns, hub.maxConns)

	hub2 := NewHub(-5)
	assert.Equal(t, DefaultMaxConns, hub2.maxConns)
}

func TestNewHub_CustomMaxConns(t *testing.T) {
	hub := NewHub(10)
	assert.Equal(t, 10, hub.maxConns)
}

func TestHub_RegisterAndClientCount(t *testing.T) {
	hub := NewHub(20)
	go hub.Run()

	server := newTestServer(t, hub)
	defer server.Close()

	conn := dialWS(t, server)
	defer conn.Close()

	// Wait for registration to be processed.
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, 1, hub.ClientCount())
}

func TestHub_UnregisterOnClose(t *testing.T) {
	hub := NewHub(20)
	go hub.Run()

	server := newTestServer(t, hub)
	defer server.Close()

	conn := dialWS(t, server)

	// Wait for registration.
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 1, hub.ClientCount())

	// Close the connection from client side.
	conn.Close()

	// Wait for unregister to be processed.
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 0, hub.ClientCount())
}

func TestHub_BroadcastToAllClients(t *testing.T) {
	hub := NewHub(20)
	go hub.Run()

	server := newTestServer(t, hub)
	defer server.Close()

	// Connect 3 clients.
	conns := make([]*websocket.Conn, 3)
	for i := range conns {
		conns[i] = dialWS(t, server)
		defer conns[i].Close()
	}

	// Wait for all registrations.
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 3, hub.ClientCount())

	// Broadcast an event.
	event := Event{
		Type:    "push",
		Payload: map[string]string{"title": "test"},
	}
	hub.Broadcast(event)

	// All clients should receive the event.
	for i, conn := range conns {
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		var received Event
		err := conn.ReadJSON(&received)
		require.NoError(t, err, "client %d failed to read", i)
		assert.Equal(t, "push", received.Type)
	}
}

func TestHub_MaxConnectionsReject(t *testing.T) {
	maxConns := 3
	hub := NewHub(maxConns)
	go hub.Run()

	server := newTestServer(t, hub)
	defer server.Close()

	// Connect up to max.
	conns := make([]*websocket.Conn, maxConns)
	for i := range conns {
		conns[i] = dialWS(t, server)
		defer conns[i].Close()
	}

	// Wait for all registrations.
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, maxConns, hub.ClientCount())

	// Try to connect one more - should be rejected.
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	extraConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err == nil {
		defer extraConn.Close()
		// The connection might be established at TCP level but the hub will close it.
		// Try to read - should get a close or error.
		extraConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		_, _, readErr := extraConn.ReadMessage()
		assert.Error(t, readErr, "expected error reading from rejected connection")
	}

	// Client count should still be at max.
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, maxConns, hub.ClientCount())
}

func TestHub_ConcurrentBroadcast(t *testing.T) {
	hub := NewHub(20)
	go hub.Run()

	server := newTestServer(t, hub)
	defer server.Close()

	conn := dialWS(t, server)
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	// Broadcast multiple events concurrently.
	var wg sync.WaitGroup
	numEvents := 10
	for i := 0; i < numEvents; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			hub.Broadcast(Event{
				Type:    "push",
				Payload: idx,
			})
		}(i)
	}
	wg.Wait()

	// Read all events.
	received := 0
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	for received < numEvents {
		var event Event
		err := conn.ReadJSON(&event)
		if err != nil {
			break
		}
		received++
	}
	assert.Equal(t, numEvents, received)
}
