package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/overseer/overseer/internal/model"
	"github.com/overseer/overseer/internal/server/middleware"
	"github.com/overseer/overseer/internal/store"
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
	hub       *ws.Hub
	jwtSecret string
	store     store.Store
}

// NewWSHandler creates a new WSHandler with the given Hub, JWT secret, and store.
func NewWSHandler(hub *ws.Hub, jwtSecret string, db store.Store) *WSHandler {
	return &WSHandler{
		hub:       hub,
		jwtSecret: jwtSecret,
		store:     db,
	}
}

// Register registers the WebSocket route on the given engine.
// The /ws endpoint uses query parameter token for authentication
// instead of the standard middleware.
func (h *WSHandler) Register(engine *gin.Engine) {
	engine.GET("/ws", h.HandleWS)
}

// HandleWS upgrades the HTTP connection to a WebSocket connection.
// Authentication is performed via:
// 1. "api_key" query parameter (for desktop clients)
// 2. "token" query parameter (JWT) or the overseer_token cookie (for Web UI)
func (h *WSHandler) HandleWS(c *gin.Context) {
	// Try desktop client api_key authentication first
	apiKey := c.Query("api_key")
	if apiKey != "" {
		device, err := h.store.GetDeviceByKey(apiKey, model.DeviceTypeDesktop)
		if err != nil || device == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "unauthorized: invalid api_key",
			})
			return
		}
		// Desktop client authenticated successfully
		h.upgradeConnection(c)
		return
	}

	// Fall through to JWT/cookie authentication (Web UI)
	tokenStr := c.Query("token")
	if tokenStr == "" {
		// Try cookie
		if cookie, err := c.Cookie("overseer_token"); err == nil && cookie != "" {
			tokenStr = cookie
		}
	}

	if tokenStr == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "unauthorized: missing token",
		})
		return
	}

	// Validate JWT token
	claims := &middleware.JWTClaims{}
	parsed, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(h.jwtSecret), nil
	})
	if err != nil || !parsed.Valid {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "unauthorized: invalid token",
		})
		return
	}

	h.upgradeConnection(c)
}

// upgradeConnection upgrades the HTTP connection to WebSocket and registers the client.
func (h *WSHandler) upgradeConnection(c *gin.Context) {
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
