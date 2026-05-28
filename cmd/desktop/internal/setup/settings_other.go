//go:build !windows

package setup

import (
	"github.com/overseer/overseer/cmd/desktop/internal/config"
)

// ShowSettingsDialog is a no-op on non-Windows platforms for now.
func ShowSettingsDialog(cfg *config.Config) (newServerURL string, reRegister bool, err error) {
	// Terminal-based settings not implemented yet
	return "", false, nil
}
