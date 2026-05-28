package main

import (
	"flag"
	"log"
	"os"

	"github.com/overseer/overseer/cmd/desktop/internal/api"
	"github.com/overseer/overseer/cmd/desktop/internal/config"
	"github.com/overseer/overseer/cmd/desktop/internal/notifier"
	"github.com/overseer/overseer/cmd/desktop/internal/setup"
	"github.com/overseer/overseer/cmd/desktop/internal/tray"
	"github.com/overseer/overseer/cmd/desktop/internal/wsclient"
)

// version is set by -ldflags at build time.
var version = "dev"

func main() {
	// Optional CLI flags (for advanced users / scripting)
	serverURL := flag.String("server", "", "Overseer server URL")
	registerToken := flag.String("token", "", "Registration token")
	deviceName := flag.String("name", "", "Device name")
	showVersion := flag.Bool("version", false, "Show version")
	flag.Parse()

	if *showVersion {
		log.Printf("overseer-desktop %s\n", version)
		os.Exit(0)
	}

	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Printf("[desktop] overseer-desktop %s starting...", version)

	// Load or create config
	cfg, err := config.Load()
	if err != nil {
		log.Printf("[desktop] config load error: %v", err)
		cfg = &config.Config{}
	}

	// If CLI flags provided, use them (for scripting / CI)
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
		if err := config.Save(cfg); err != nil {
			log.Fatalf("[desktop] failed to save config: %v", err)
		}
		log.Printf("[desktop] registered as %q", device.Name)
	}

	// If not configured yet, show setup dialog
	if cfg.ServerURL == "" || cfg.APIKey == "" {
		log.Println("[desktop] first-time setup required, showing dialog...")

		result, err := setup.ShowDialog()
		if err != nil {
			log.Fatalf("[desktop] setup: %v", err)
		}

		if err := setup.Run(cfg, *result); err != nil {
			log.Fatalf("[desktop] setup failed: %v", err)
		}

		log.Printf("[desktop] setup complete: registered as %q", cfg.DeviceName)
	}

	// Create notifier
	n := notifier.New(cfg.ServerURL)

	// Create WebSocket client
	ws := wsclient.New(cfg.ServerURL, cfg.APIKey, func(event wsclient.PushEvent) {
		log.Printf("[desktop] notification: %s - %s", event.Title, event.Body)
		n.Show(event.Title, event.Body, event.URL)
	})

	// Start WebSocket connection in background
	go ws.Connect()

	// Run system tray (this blocks until quit)
	t := tray.New(cfg, ws, n)
	t.Run()

	// Cleanup after tray exits
	log.Println("[desktop] shutting down...")
	ws.Close()
}
