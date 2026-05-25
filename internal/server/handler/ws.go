package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/overseer/overseer/internal/ws"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// WSHandler handles WebSocket upgrade requests.
type WSHandler struct {
	hub    *ws.Hub
	apiKey string
}

// NewWSHandler creates a new WSHandler with the given Hub and API key.
func NewWSHandler(hub *ws.Hub, apiKey string) *WSHandler {
	return &WSHandler{
		hub:    hub,
		apiKey: apiKey,
	}
}

// Register registers the WebSocket route on the given engine.
// The /ws endpoint uses query parameter token for authentication
// instead of the standard X-API-Key header middleware.
func (h *WSHandler) Register(engine *gin.Engine) {
	engine.GET("/ws", h.HandleWS)
}

// HandleWS upgrades the HTTP connection to a WebSocket connection.
// Authentication is performed via the "token" query parameter.
func (h *WSHandler) HandleWS(c *gin.Context) {
	token := c.Query("token")
	if token == "" || token != h.apiKey {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "unauthorized: invalid or missing token",
		})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		// Upgrade failure is handled by the upgrader which writes the HTTP error.
		return
	}

	client := ws.NewClient(h.hub, conn)
	h.hub.Register(client)

	go client.WritePump()
	go client.ReadPump()
}
