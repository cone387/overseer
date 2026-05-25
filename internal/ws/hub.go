package ws

import (
	"sync"
)

const (
	// DefaultMaxConns is the default maximum number of concurrent WebSocket connections.
	DefaultMaxConns = 20
)

// Event represents a WebSocket event broadcast to connected clients.
type Event struct {
	Type    string      `json:"type"`    // "push", "reminder_change"
	Payload interface{} `json:"payload"`
}

// Hub maintains the set of active WebSocket clients and broadcasts events to them.
type Hub struct {
	// Registered clients.
	clients map[*Client]bool

	// Channel for broadcasting events to all clients.
	broadcast chan Event

	// Register requests from clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client

	// Maximum number of concurrent connections.
	maxConns int

	mu sync.RWMutex
}

// NewHub creates a new Hub with the specified maximum connections.
// If maxConns <= 0, DefaultMaxConns is used.
func NewHub(maxConns int) *Hub {
	if maxConns <= 0 {
		maxConns = DefaultMaxConns
	}
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan Event, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		maxConns:   maxConns,
	}
}

// Run starts the hub's event loop processing register, unregister, and broadcast channels.
// This should be run as a goroutine.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if len(h.clients) >= h.maxConns {
				h.mu.Unlock()
				// Reject the connection: close the client's send channel and connection.
				close(client.send)
				client.conn.Close()
				continue
			}
			h.clients[client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()

		case event := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- event:
				default:
					// Client's send buffer is full; remove it.
					h.mu.RUnlock()
					h.mu.Lock()
					if _, ok := h.clients[client]; ok {
						delete(h.clients, client)
						close(client.send)
					}
					h.mu.Unlock()
					h.mu.RLock()
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast sends an event to all connected clients.
func (h *Hub) Broadcast(event Event) {
	h.broadcast <- event
}

// ClientCount returns the current number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Register adds a client to the hub.
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister removes a client from the hub.
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}
