//go:build linux

package autostart

import (
	"fmt"
	"os"
	"path/filepath"
)

const desktopFileName = "overseer-desktop.desktop"

func desktopFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "autostart", desktopFileName)
}

func isEnabled() bool {
	_, err := os.Stat(desktopFilePath())
	return err == nil
}

func enable() error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}

	content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Overseer Desktop
Exec=%s
Hidden=false
NoDisplay=false
X-GNOME-Autostart-enabled=true
Comment=Overseer Desktop Notification Client
`, exePath)

	dir := filepath.Dir(desktopFilePath())
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(desktopFilePath(), []byte(content), 0644)
}

func disable() error {
	return os.Remove(desktopFilePath())
}
