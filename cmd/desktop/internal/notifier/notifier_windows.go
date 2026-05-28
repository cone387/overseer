//go:build windows

package notifier

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/go-toast/toast"
)

const appID = "Overseer"

var (
	setupOnce sync.Once
	iconPath  string
)

// Show displays a basic toast notification.
func (n *Notifier) Show(title, body, clickURL string) {
	n.showWithLevel(title, body, clickURL, "default")
}

// showWithLevel displays a toast with level-appropriate audio.
func (n *Notifier) showWithLevel(title, body, clickURL, level string) {
	n.mu.Lock()
	muted := n.muted
	n.mu.Unlock()

	if muted {
		log.Printf("[notifier] muted, skipping: %s", title)
		return
	}

	setupOnce.Do(func() {
		iconPath = ensureIconFile()
		_ = ensureStartMenuShortcut()
	})

	if clickURL == "" {
		clickURL = n.serverURL
	}

	// Map level to audio and build notification
	notification := toast.Notification{
		AppID:               appID,
		Title:               title,
		Message:             body,
		ActivationType:      "protocol",
		ActivationArguments: clickURL,
	}

	switch level {
	case "critical":
		notification.Audio = toast.LoopingAlarm
	case "timeSensitive":
		notification.Audio = toast.Reminder
	case "passive":
		notification.Audio = toast.Silent
	default:
		notification.Audio = toast.Default
	}

	if iconPath != "" {
		notification.Icon = iconPath
	}

	if err := notification.Push(); err != nil {
		log.Printf("[notifier] toast error: %v", err)
	}
}

func ensureIconFile() string {
	dir := filepath.Join(os.Getenv("LOCALAPPDATA"), "overseer-desktop")
	_ = os.MkdirAll(dir, 0755)
	iconFile := filepath.Join(dir, "icon.png")

	if _, err := os.Stat(iconFile); err == nil {
		return iconFile
	}

	ps := fmt.Sprintf(`
Add-Type -AssemblyName System.Drawing
$bmp = New-Object System.Drawing.Bitmap(64, 64)
$g = [System.Drawing.Graphics]::FromImage($bmp)
$g.SmoothingMode = 'AntiAlias'
$g.Clear([System.Drawing.Color]::Transparent)
$brush = New-Object System.Drawing.SolidBrush([System.Drawing.Color]::FromArgb(255, 66, 133, 244))
$g.FillEllipse($brush, 4, 4, 56, 56)
$font = New-Object System.Drawing.Font('Segoe UI', 28, [System.Drawing.FontStyle]::Bold)
$sf = New-Object System.Drawing.StringFormat
$sf.Alignment = 'Center'
$sf.LineAlignment = 'Center'
$g.DrawString('O', $font, [System.Drawing.Brushes]::White, (New-Object System.Drawing.RectangleF(0, 0, 64, 64)), $sf)
$g.Dispose()
$bmp.Save('%s', [System.Drawing.Imaging.ImageFormat]::Png)
$bmp.Dispose()
`, strings.ReplaceAll(iconFile, `\`, `\\`))

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", ps)
	if err := cmd.Run(); err != nil {
		log.Printf("[notifier] icon generation failed: %v", err)
		return ""
	}
	return iconFile
}

func ensureStartMenuShortcut() error {
	startMenuDir := filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs")
	shortcutPath := filepath.Join(startMenuDir, "Overseer.lnk")

	if _, err := os.Stat(shortcutPath); err == nil {
		return nil
	}

	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	exePath, _ = filepath.Abs(exePath)

	ps := fmt.Sprintf(`
$WshShell = New-Object -ComObject WScript.Shell
$Shortcut = $WshShell.CreateShortcut('%s')
$Shortcut.TargetPath = '%s'
$Shortcut.Description = 'Overseer Desktop'
$Shortcut.Save()
`, escapePSPath(shortcutPath), escapePSPath(exePath))

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", ps)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("shortcut: %w (%s)", err, string(output))
	}
	return nil
}

func escapePSPath(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
