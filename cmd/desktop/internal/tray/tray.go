package tray

import (
	"fmt"
	"log"
	"os/exec"
	"runtime"

	"github.com/overseer/overseer/cmd/desktop/internal/config"
	"github.com/overseer/overseer/cmd/desktop/internal/wsclient"
)

// Tray represents the system tray integration.
// For the MVP, this is a lightweight wrapper that logs status.
// A full systray implementation (using github.com/getlantern/systray) can be added later.
type Tray struct {
	cfg *config.Config
	ws  *wsclient.Client
}

// New creates a new Tray instance.
func New(cfg *config.Config, ws *wsclient.Client) *Tray {
	return &Tray{cfg: cfg, ws: ws}
}

// Run starts the tray. For the MVP, this just logs the status.
// In a full implementation, this would initialize the system tray icon and menu.
func (t *Tray) Run() {
	log.Printf("[tray] overseer desktop client running")
	log.Printf("[tray] server: %s", t.cfg.ServerURL)
	log.Printf("[tray] device: %s (%s)", t.cfg.DeviceName, t.cfg.DeviceID)
	log.Printf("[tray] press Ctrl+C to quit")
}

// Quit cleans up the tray resources.
func (t *Tray) Quit() {
	log.Println("[tray] quit")
}

// OpenWebUI opens the Overseer Web UI in the default browser.
func (t *Tray) OpenWebUI() {
	url := t.cfg.ServerURL
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}

	if err := cmd.Start(); err != nil {
		log.Printf("[tray] failed to open browser: %v", err)
	}
}

// Status returns a human-readable connection status string.
func (t *Tray) Status() string {
	if t.ws.Connected() {
		return fmt.Sprintf("Connected to %s", t.cfg.ServerURL)
	}
	return "Disconnected"
}
