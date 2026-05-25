package ws

import (
	"encoding/json"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// IdleTimeout is the maximum time a client can be idle (no messages received)
	// before the connection is closed.
	IdleTimeout = 60 * time.Second

	// writeWait is the time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// pongWait is the time allowed to read the next pong message from the peer.
	pongWait = IdleTimeout

	// pingPeriod sends pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// maxMessageSize is the maximum message size allowed from peer.
	maxMessageSize = 512

	// sendBufferSize is the size of the client's outbound message buffer.
	sendBufferSize = 256
)

// Client represents a single WebSocket connection managed by the Hub.
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan Event
}

// NewClient creates a new Client associated with the given hub and connection.
func NewClient(hub *Hub, conn *websocket.Conn) *Client {
	return &Client{
		hub:  hub,
		conn: conn,
		send: make(chan Event, sendBufferSize),
	}
}

// ReadPump pumps messages from the WebSocket connection to the hub.
// It monitors for incoming messages (including pings) and enforces the idle timeout.
// If no message is received within IdleTimeout, the connection is closed.
func (c *Client) ReadPump() {
	defer func() {
		c.hub.Unregister(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		// Any message received resets the idle timeout.
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
	}
}

// WritePump pumps messages from the hub to the WebSocket connection.
// It sends events from the send channel and periodically pings the client.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case event, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			data, err := json.Marshal(event)
			if err != nil {
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
