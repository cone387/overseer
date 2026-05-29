package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/overseer/overseer/cmd/desktop/internal/api"
	"github.com/overseer/overseer/cmd/desktop/internal/cache"
	"github.com/overseer/overseer/cmd/desktop/internal/config"
	"github.com/overseer/overseer/cmd/desktop/internal/grouper"
	"github.com/overseer/overseer/cmd/desktop/internal/locale"
	"github.com/overseer/overseer/cmd/desktop/internal/notifier"
	"github.com/overseer/overseer/cmd/desktop/internal/setup"
	"github.com/overseer/overseer/cmd/desktop/internal/tray"
	"github.com/overseer/overseer/cmd/desktop/internal/unread"
	"github.com/overseer/overseer/cmd/desktop/internal/updater"
	"github.com/overseer/overseer/cmd/desktop/internal/wsclient"
)

// version is set by -ldflags at build time.
var version = "dev"

func main() {
	// Optional CLI flags
	serverURL := flag.String("server", "", "Overseer server URL")
	registerToken := flag.String("token", "", "Registration token")
	deviceName := flag.String("name", "", "Device name")
	showVersion := flag.Bool("version", false, "Show version")
	flag.Parse()

	if *showVersion {
		fmt.Printf("overseer-desktop %s\n", version)
		os.Exit(0)
	}

	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Printf("[desktop] overseer-desktop %s starting... (lang=%s)", version, locale.Lang)

	// Load config
	cfg, err := config.Load()
	if err != nil {
		log.Printf("[desktop] config load error: %v", err)
		cfg = &config.Config{}
	}

	// CLI registration mode
	if *serverURL != "" && *registerToken != "" {
		name := *deviceName
		if name == "" {
			hostname, _ := os.Hostname()
			name = "Desktop " + hostname
		}
		cfg.ServerURL = *serverURL
		device, err := api.Register(cfg.ServerURL, name, *registerToken)
		if err != nil {
			log.Fatalf("[desktop] registration failed: %v", err)
		}
		cfg.APIKey = device.DeviceKey
		cfg.DeviceID = device.ID
		cfg.DeviceName = device.Name
		_ = config.Save(cfg)
		log.Printf("[desktop] registered as %q", device.Name)
	}

	// First-time setup dialog
	if cfg.ServerURL == "" || cfg.APIKey == "" {
		result, err := setup.ShowDialog()
		if err != nil {
			log.Fatalf("[desktop] setup: %v", err)
		}
		if err := setup.Run(cfg, *result); err != nil {
			log.Fatalf("[desktop] setup failed: %v", err)
		}
		log.Printf("[desktop] registered as %q", cfg.DeviceName)
	}

	// Initialize notification cache
	notifCache, err := cache.New()
	if err != nil {
		log.Printf("[desktop] cache init error (continuing without cache): %v", err)
	}

	// Create notifier
	n := notifier.New(cfg.ServerURL)

	// Create message grouper (batches rapid notifications from same source)
	msgGrouper := grouper.New(func(title, body, url, channel, source, level, msgID string) {
		n.ShowRich(title, body, url, channel, source, level, msgID)
	})

	// Declare tracker (initialized after tray creation)
	var tracker *unread.Tracker

	// Create WebSocket client
	ws := wsclient.New(cfg.ServerURL, cfg.APIKey, func(event wsclient.PushEvent) {
		log.Printf("[desktop] notification: [%s] %s - %s", event.Channel, event.Title, event.Body)

		// Cache the notification
		if notifCache != nil {
			var expiresAt *time.Time
			if event.ExpiresAt != "" {
				if t, err := time.Parse(time.RFC3339, event.ExpiresAt); err == nil {
					expiresAt = &t
				}
			}

			_ = notifCache.Add(cache.Entry{
				ID:         event.ID,
				Title:      event.Title,
				Body:       event.Body,
				URL:        event.URL,
				Channel:    event.Channel,
				Source:     event.Source,
				ReceivedAt: time.Now(),
				Unread:     true,
				ExpiresAt:  expiresAt,
			})
		}

		// Update unread tracker
		if tracker != nil {
			tracker.OnPush(event.ID)
		}

		// Route through grouper (groups rapid notifications from same source)
		msgGrouper.Ingest(grouper.PushEvent{
			ID:      event.ID,
			Source:  event.Source,
			Channel: event.Channel,
			Title:   event.Title,
			Body:    event.Body,
			URL:     event.URL,
			Level:   event.Level,
		})
	})

	// Handle auth failure (401) — clear config and re-register
	ws.SetOnAuthFail(func() {
		log.Println("[desktop] auth failed (401), clearing credentials...")
		cfg.APIKey = ""
		_ = config.Save(cfg)
		// The WS client will stop retrying after this
		// User needs to restart the app to re-register
		// (In a full implementation, we'd re-show the dialog here)
	})

	// Auto-update check
	go func() {
		result := updater.Check(version, cfg.LastUpdateCheck)
		if result != nil && result.Available {
			log.Printf("[updater] new version available: %s", result.Version)
			n.Show(
				locale.T("notify.update_title"),
				fmt.Sprintf(locale.T("notify.update_message"), result.Version),
				result.DownloadURL,
			)
		}
		// Update last check time
		cfg.LastUpdateCheck = time.Now()
		_ = config.Save(cfg)
	}()

	// Start WebSocket
	go ws.Connect()

	// Run system tray (blocks)
	t := tray.New(cfg, ws, n, notifCache)

	// Create unread tracker
	tracker = unread.New(notifCache, t.UpdateUnread)

	// Wire ack_sync events to tracker
	ws.SetOnAckSync(func(messageID string) {
		tracker.OnAckSync(messageID)
	})

	t.Run()

	// Cleanup
	log.Println("[desktop] shutting down...")
	ws.Close()
	if notifCache != nil {
		notifCache.Close()
	}
}
