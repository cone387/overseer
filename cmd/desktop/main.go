package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/overseer/overseer/cmd/desktop/internal/api"
	"github.com/overseer/overseer/cmd/desktop/internal/config"
	"github.com/overseer/overseer/cmd/desktop/internal/notifier"
	"github.com/overseer/overseer/cmd/desktop/internal/tray"
	"github.com/overseer/overseer/cmd/desktop/internal/wsclient"
)

func main() {
	serverURL := flag.String("server", "", "Overseer server URL (e.g. http://localhost:9721)")
	registerToken := flag.String("token", "", "Registration token for first-time setup")
	deviceName := flag.String("name", "", "Device name for registration")
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("[desktop] starting overseer desktop client...")

	// Load or create config
	cfg, err := config.Load()
	if err != nil {
		log.Printf("[desktop] config load error (will use defaults): %v", err)
		cfg = &config.Config{}
	}

	// Override server URL from flag if provided
	if *serverURL != "" {
		cfg.ServerURL = *serverURL
		if err := config.Save(cfg); err != nil {
			log.Printf("[desktop] failed to save config: %v", err)
		}
	}

	// If no server URL configured, prompt and exit
	if cfg.ServerURL == "" {
		fmt.Println("No server URL configured. Use --server flag to set it:")
		fmt.Println("  overseer-desktop --server http://localhost:9721 --token YOUR_TOKEN")
		os.Exit(1)
	}

	// If no API key, register the device
	if cfg.APIKey == "" {
		if *registerToken == "" {
			fmt.Println("No API key found. First-time registration requires --token flag:")
			fmt.Println("  overseer-desktop --server http://localhost:9721 --token YOUR_TOKEN")
			os.Exit(1)
		}

		name := *deviceName
		if name == "" {
			hostname, _ := os.Hostname()
			if hostname != "" {
				name = "Desktop " + hostname
			} else {
				name = "Desktop Client"
			}
		}

		log.Printf("[desktop] registering device %q with server %s...", name, cfg.ServerURL)
		device, err := api.Register(cfg.ServerURL, name, *registerToken)
		if err != nil {
			log.Fatalf("[desktop] registration failed: %v", err)
		}

		cfg.APIKey = device.DeviceKey
		cfg.DeviceID = device.ID
		cfg.DeviceName = device.Name
		if err := config.Save(cfg); err != nil {
			log.Fatalf("[desktop] failed to save config after registration: %v", err)
		}
		log.Printf("[desktop] registered successfully as %q (id=%s)", device.Name, device.ID)
	}

	// Create notifier
	n := notifier.New(cfg.ServerURL)

	// Create WebSocket client
	ws := wsclient.New(cfg.ServerURL, cfg.APIKey, func(event wsclient.PushEvent) {
		log.Printf("[desktop] notification: %s - %s", event.Title, event.Body)
		n.Show(event.Title, event.Body, event.URL)
	})

	// Create tray (manages lifecycle)
	t := tray.New(cfg, ws)

	// Start WebSocket connection
	go ws.Connect()

	// Start system tray (blocks on some platforms)
	go t.Run()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[desktop] shutting down...")
	ws.Close()
	t.Quit()
}
