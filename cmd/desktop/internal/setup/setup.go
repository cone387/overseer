package setup

import (
	"fmt"

	"github.com/overseer/overseer/cmd/desktop/internal/api"
	"github.com/overseer/overseer/cmd/desktop/internal/config"
)

// Result holds the setup dialog result.
type Result struct {
	ServerURL string
	Token     string
	Name      string
}

// Run performs the first-time setup: shows a dialog, registers the device, and saves config.
func Run(cfg *config.Config, result Result) error {
	cfg.ServerURL = result.ServerURL

	// Register with the server
	device, err := api.Register(cfg.ServerURL, result.Name, result.Token)
	if err != nil {
		return fmt.Errorf("registration failed: %w", err)
	}

	cfg.APIKey = device.DeviceKey
	cfg.DeviceID = device.ID
	cfg.DeviceName = device.Name

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	return nil
}
