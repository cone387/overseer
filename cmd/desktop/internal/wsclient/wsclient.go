package wsclient

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// PushEvent represents a push notification received via WebSocket.
type PushEvent struct {
	ID        string `json:"id"`
	Source    string `json:"source"`
	Channel   string `json:"channel"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	URL       string `json:"url"`
	Icon      string `json:"icon"`
	Level     string `json:"level"`
	Status    string `json:"status"`
	Time      string `json:"time"`
	ExpiresAt string `json:"expires_at"`
}

// wsEvent is the raw WebSocket event envelope.
type wsEvent struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// OnPushFunc is the callback invoked when a push event is received.
type OnPushFunc func(event PushEvent)

// OnAckSyncFunc is the callback invoked when an ack_sync event is received.
type OnAckSyncFunc func(messageID string)

// OnAuthFailFunc is the callback invoked when WebSocket auth fails (401).
type OnAuthFailFunc func()

// Client manages the WebSocket connection to the Overseer backend.
type Client struct {
	serverURL  string
	apiKey     string
	onPush     OnPushFunc
	onAckSync  OnAckSyncFunc
	onAuthFail OnAuthFailFunc

	conn   *websocket.Conn
	mu     sync.Mutex
	closed bool
	done   chan struct{}
}

// New creates a new WebSocket client.
func New(serverURL, apiKey string, onPush OnPushFunc) *Client {
	return &Client{
		serverURL: serverURL,
		apiKey:    apiKey,
		onPush:    onPush,
		done:      make(chan struct{}),
	}
}

// SetOnAuthFail sets the callback for authentication failures.
func (c *Client) SetOnAuthFail(fn OnAuthFailFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onAuthFail = fn
}

// SetOnAckSync sets the callback for ack_sync events.
func (c *Client) SetOnAckSync(fn OnAckSyncFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onAckSync = fn
}

// Connected returns true if the WebSocket connection is active.
func (c *Client) Connected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn != nil && !c.closed
}

// Connect establishes the WebSocket connection with exponential backoff retry.
// This method blocks until Close() is called.
func (c *Client) Connect() {
	backoff := time.Second
	maxBackoff := 60 * time.Second

	for {
		select {
		case <-c.done:
			return
		default:
		}

		err := c.dial()
		if err != nil {
			log.Printf("[ws] connection failed: %v (retry in %v)", err, backoff)

			select {
			case <-c.done:
				return
			case <-time.After(backoff):
			}

			// Exponential backoff: 1s -> 2s -> 4s -> 8s -> ... -> 60s
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		// Connected successfully — reset backoff
		backoff = time.Second
		log.Println("[ws] connected")

		// Read messages until disconnection
		c.readLoop()

		log.Println("[ws] disconnected")
	}
}

// Close terminates the WebSocket connection and stops reconnection attempts.
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return
	}
	c.closed = true
	close(c.done)

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

// Reconnect forces a reconnection by closing the current connection.
func (c *Client) Reconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

// dial establishes a single WebSocket connection.
func (c *Client) dial() error {
	// Build WebSocket URL
	u, err := url.Parse(c.serverURL)
	if err != nil {
		return err
	}

	// Convert http(s) to ws(s)
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	default:
		u.Scheme = "ws"
	}
	u.Path = "/ws"
	q := u.Query()
	q.Set("api_key", c.apiKey)
	u.RawQuery = q.Encode()

	conn, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		// Check for 401 Unauthorized
		if resp != nil && resp.StatusCode == 401 {
			c.mu.Lock()
			fn := c.onAuthFail
			c.mu.Unlock()
			if fn != nil {
				fn()
			}
			return fmt.Errorf("unauthorized (401)")
		}
		return err
	}

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	return nil
}

// readLoop reads messages from the WebSocket until an error occurs.
func (c *Client) readLoop() {
	for {
		select {
		case <-c.done:
			return
		default:
		}

		c.mu.Lock()
		conn := c.conn
		c.mu.Unlock()

		if conn == nil {
			return
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			return
		}

		var event wsEvent
		if err := json.Unmarshal(message, &event); err != nil {
			log.Printf("[ws] failed to parse event: %v", err)
			continue
		}

		if event.Type == "push" {
			var push PushEvent
			if err := json.Unmarshal(event.Payload, &push); err != nil {
				log.Printf("[ws] failed to parse push payload: %v", err)
				continue
			}
			if c.onPush != nil {
				c.onPush(push)
			}
		} else if event.Type == "ack_sync" {
			var ackPayload struct {
				MessageID string `json:"message_id"`
			}
			if err := json.Unmarshal(event.Payload, &ackPayload); err != nil {
				log.Printf("[ws] failed to parse ack_sync payload: %v", err)
				continue
			}
			c.mu.Lock()
			fn := c.onAckSync
			c.mu.Unlock()
			if fn != nil {
				fn(ackPayload.MessageID)
			}
		}
	}
}
