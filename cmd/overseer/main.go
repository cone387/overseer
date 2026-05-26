package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	overseer "github.com/overseer/overseer"
	"github.com/overseer/overseer/internal/aggregator"
	"github.com/overseer/overseer/internal/config"
	"github.com/overseer/overseer/internal/escalator"
	"github.com/overseer/overseer/internal/model"
	"github.com/overseer/overseer/internal/pusher"
	"github.com/overseer/overseer/internal/router"
	"github.com/overseer/overseer/internal/scheduler"
	"github.com/overseer/overseer/internal/server/handler"
	"github.com/overseer/overseer/internal/server/middleware"
	"github.com/overseer/overseer/internal/store"
	"github.com/overseer/overseer/internal/template"
	"github.com/overseer/overseer/internal/ws"
)

func main() {
	// Parse command-line flags
	configPath := flag.String("config", "", "path to configuration file")
	flag.Parse()

	log.Println("[overseer] starting...")

	// Load and validate configuration
	loader := config.NewYAMLLoader()
	cfg, err := loader.Load(*configPath)
	if err != nil {
		log.Fatalf("[overseer] failed to load config: %v", err)
	}
	if err := loader.Validate(cfg); err != nil {
		log.Fatalf("[overseer] config validation failed: %v", err)
	}
	log.Println("[overseer] configuration loaded and validated")

	// Auto-generate JWT secret if not configured
	jwtSecret := cfg.Server.JWTSecret
	if jwtSecret == "" {
		secretBytes := make([]byte, 32)
		if _, err := rand.Read(secretBytes); err != nil {
			log.Fatalf("[overseer] failed to generate JWT secret: %v", err)
		}
		jwtSecret = base64.URLEncoding.EncodeToString(secretBytes)
		log.Println("[overseer] WARNING: jwt_secret not configured, using auto-generated secret (sessions will not persist across restarts)")
	}

	// Initialize SQLite store and run migrations
	db, err := store.NewSQLiteStore("overseer.db")
	if err != nil {
		log.Fatalf("[overseer] failed to open database: %v", err)
	}
	if err := db.Migrate(); err != nil {
		log.Fatalf("[overseer] failed to run migrations: %v", err)
	}
	log.Println("[overseer] database initialized")

	// Initialize Router
	msgRouter, err := router.NewRouter(cfg.Rules, cfg.Channels)
	if err != nil {
		log.Fatalf("[overseer] failed to create router: %v", err)
	}

	// Initialize Template Engine
	tmplEngine, err := template.NewEngine(cfg.Templates)
	if err != nil {
		log.Fatalf("[overseer] failed to create template engine: %v", err)
	}

	// Initialize Aggregator
	agg := aggregator.NewAggregator(cfg.Aggregator)

	// Initialize DND Scheduler
	dndScheduler, err := scheduler.NewDNDScheduler(cfg.DND, cfg.Aggregator.MaxPending)
	if err != nil {
		log.Fatalf("[overseer] failed to create DND scheduler: %v", err)
	}

	// Initialize Bark Pusher
	barkPusher := pusher.NewBarkPusher(cfg.Bark)

	// Initialize WebSocket Hub
	wsHub := ws.NewHub(20)
	go wsHub.Run()

	// Helper: get default device key from database
	getDefaultDeviceKey := func() string {
		device, err := db.GetDefaultDevice()
		if err == nil && device != nil {
			log.Printf("[overseer] push: using default device %q key=%q", device.Name, device.DeviceKey)
			return device.DeviceKey
		}
		// Fallback: get first device from DB
		devices, err := db.ListDevices()
		if err == nil && len(devices) > 0 {
			log.Printf("[overseer] push: using first device %q key=%q", devices[0].Name, devices[0].DeviceKey)
			return devices[0].DeviceKey
		}
		log.Println("[overseer] push: WARNING no devices configured in database")
		return ""
	}

	// Define the push function used by escalator and reminder scheduler
	pushFunc := func(msg *model.Message) error {
		ch, tmplName := msgRouter.Route(msg)
		msg.Channel = ch.Name

		// Render template if specified
		if tmplName != "" {
			rendered, _ := tmplEngine.Render(tmplName, msg)
			msg.Body = rendered
		}

		// Push to channel
		req := pusher.PushRequest{
			Title: msg.Title,
			Body:  msg.Body,
		}
		results := barkPusher.PushToChannel(context.Background(), req, *ch, getDefaultDeviceKey())

		// Determine overall success
		anySuccess := false
		var lastErr string
		for _, r := range results {
			if r.Success {
				anySuccess = true
			} else {
				lastErr = r.Error
			}
		}

		if anySuccess {
			msg.Status = model.StatusSuccess
			now := time.Now()
			msg.PushedAt = &now
			_ = db.UpdateMessageStatus(msg.ID, model.StatusSuccess, "")
		} else {
			msg.Status = model.StatusFailed
			msg.FailReason = lastErr
			_ = db.UpdateMessageStatus(msg.ID, model.StatusFailed, lastErr)
		}

		// Broadcast push event via WebSocket
		wsHub.Broadcast(ws.Event{
			Type:    "push",
			Payload: msg,
		})

		if !anySuccess {
			return &pushError{msg: lastErr}
		}
		return nil
	}

	// Initialize Escalator
	esc := escalator.NewEscalator(cfg.Escalation, pushFunc)

	// Initialize Reminder Scheduler
	reminderScheduler := scheduler.NewReminderScheduler(db, pushFunc)
	if err := reminderScheduler.Start(); err != nil {
		log.Fatalf("[overseer] failed to start reminder scheduler: %v", err)
	}
	log.Println("[overseer] reminder scheduler started")

	// Define the message handler pipeline
	messageHandler := func(msg *model.Message) error {
		log.Printf("[overseer] messageHandler: processing message %s title=%q", msg.ID, msg.Title)
		// Route message
		ch, tmplName := msgRouter.Route(msg)
		msg.Channel = ch.Name
		log.Printf("[overseer] messageHandler: routed to channel=%q level=%q", ch.Name, ch.Level)

		// Render template
		if tmplName != "" {
			rendered, _ := tmplEngine.Render(tmplName, msg)
			msg.Body = rendered
		}

		// Save message to database
		if err := db.SaveMessage(msg); err != nil {
			return err
		}

		// Aggregation check
		result := agg.Process(msg, ch.Level)
		log.Printf("[overseer] messageHandler: aggregation result=%d for message %s", result.Action, msg.ID)
		switch result.Action {
		case aggregator.ActionDedupe:
			log.Printf("[overseer] messageHandler: message %s deduplicated, skipping push", msg.ID)
			_ = db.UpdateMessageStatus(msg.ID, model.StatusSuccess, "deduplicated")
			return nil
		case aggregator.ActionRateLimit:
			log.Printf("[overseer] messageHandler: message %s rate-limited", msg.ID)
			return nil
		case aggregator.ActionBatch:
			// Batched - push summary
			msg.Body = result.Summary
		case aggregator.ActionPass:
			// Continue with push
		}

		// DND check
		if dndScheduler.ShouldDefer(ch.Level, time.Now()) {
			dndScheduler.Enqueue(msg)
			return nil
		}

		// Push to channel
		req := pusher.PushRequest{
			Title: msg.Title,
			Body:  msg.Body,
		}
		results := barkPusher.PushToChannel(context.Background(), req, *ch, getDefaultDeviceKey())

		// Determine overall success
		anySuccess := false
		var lastErr string
		for _, r := range results {
			if r.Success {
				anySuccess = true
			} else {
				lastErr = r.Error
			}
		}

		if anySuccess {
			msg.Status = model.StatusSuccess
			now := time.Now()
			msg.PushedAt = &now
			_ = db.UpdateMessageStatus(msg.ID, model.StatusSuccess, "")
		} else {
			msg.Status = model.StatusFailed
			msg.FailReason = lastErr
			_ = db.UpdateMessageStatus(msg.ID, model.StatusFailed, lastErr)
		}

		// Broadcast push event via WebSocket
		wsHub.Broadcast(ws.Event{
			Type:    "push",
			Payload: msg,
		})

		// Track for escalation if enabled
		if cfg.Escalation.Enabled {
			esc.Track(msg)
		}

		if !anySuccess {
			return &pushError{msg: lastErr}
		}
		return nil
	}

	// Test push handler (no DB recording)
	testPushHandler := func(msg *model.Message) error {
		ch, tmplName := msgRouter.Route(msg)
		msg.Channel = ch.Name

		if tmplName != "" {
			rendered, _ := tmplEngine.Render(tmplName, msg)
			msg.Body = rendered
		}

		req := pusher.PushRequest{
			Title: msg.Title,
			Body:  msg.Body,
		}
		results := barkPusher.PushToChannel(context.Background(), req, *ch, getDefaultDeviceKey())

		for _, r := range results {
			if r.Success {
				return nil
			}
		}
		if len(results) > 0 {
			return &pushError{msg: results[0].Error}
		}
		return &pushError{msg: "no devices configured"}
	}

	// Setup Gin engine and routes
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())

	// Health check - no auth
	healthHandler := handler.NewHealthHandler()
	healthHandler.Register(engine)

	// Auth endpoints - no auth required
	authHandler := handler.NewAuthHandler(db, jwtSecret)
	authHandler.RegisterRoutes(engine)

	// WebSocket - JWT token-based auth via query param
	wsHandler := handler.NewWSHandler(wsHub, jwtSecret)
	wsHandler.Register(engine)

	// Authenticated API routes (accepts JWT or API Key)
	authGroup := engine.Group("/")
	authGroup.Use(middleware.APIKeyOrJWTAuth(db, jwtSecret))

	// Webhook endpoint
	webhookHandler := handler.NewWebhookHandler(messageHandler)
	authGroup.POST("/webhook/:source", webhookHandler.HandleWebhook)

	// Push API
	pushHandler := handler.NewPushHandler(messageHandler, testPushHandler)
	api := authGroup.Group("/api")
	api.POST("/push", pushHandler.HandlePush)
	api.POST("/push/test", pushHandler.HandleTestPush)

	// Auth protected routes (change-password, me)
	authHandler.RegisterProtectedRoutes(api)

	// API Keys management (requires JWT auth)
	apiKeysHandler := handler.NewAPIKeysHandler(db)
	apiKeysHandler.RegisterRoutes(api)

	// History API
	historyHandler := handler.NewHistoryHandler(db)
	historyHandler.RegisterRoutes(api)

	// Stats API
	statsHandler := handler.NewStatsHandler(db)
	statsHandler.RegisterRoutes(api)

	// Reminder API
	reminderHandler := handler.NewReminderHandler(db, reminderScheduler.Schedule, reminderScheduler.Cancel)
	reminderHandler.RegisterRoutes(api)

	// Device API
	deviceHandler := handler.NewDeviceHandler(db)
	deviceHandler.RegisterRoutes(api)

	// Channel API
	channelHandler := handler.NewChannelHandler(db)
	channelHandler.RegisterRoutes(api)

	// Serve embedded frontend static files with SPA fallback
	distFS, err := fs.Sub(overseer.WebDist, "web/dist")
	if err != nil {
		log.Fatalf("[overseer] failed to create sub filesystem: %v", err)
	}
	staticServer := http.FileServer(http.FS(distFS))
	engine.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// Try to serve the file directly from the embedded FS
		f, err := distFS.Open(path[1:]) // strip leading "/"
		if err == nil {
			f.Close()
			staticServer.ServeHTTP(c.Writer, c.Request)
			return
		}

		// SPA fallback: serve index.html for any unmatched route
		c.Request.URL.Path = "/"
		staticServer.ServeHTTP(c.Writer, c.Request)
	})

	// Create HTTP server
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: engine,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("[overseer] server listening on :%d", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[overseer] server error: %v", err)
		}
	}()

	log.Println("[overseer] startup complete")

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Printf("[overseer] received signal %v, shutting down...", sig)

	// Graceful shutdown with 30s timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. Shutdown HTTP server (stops accepting new requests)
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("[overseer] HTTP server shutdown error: %v", err)
	}
	log.Println("[overseer] HTTP server stopped")

	// 2. Stop Escalator (cancel pending timers)
	esc.Stop()
	log.Println("[overseer] escalator stopped")

	// 3. Stop Reminder Scheduler
	reminderScheduler.Stop()
	log.Println("[overseer] reminder scheduler stopped")

	// 4. Close database
	if err := db.Close(); err != nil {
		log.Printf("[overseer] database close error: %v", err)
	}
	log.Println("[overseer] database closed")

	log.Println("[overseer] shutdown complete")
}

// pushError is a simple error type for push failures.
type pushError struct {
	msg string
}

func (e *pushError) Error() string {
	return e.msg
}
