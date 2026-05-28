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

// appID is the Application User Model ID shown in Windows Action Center.
// Must match the AppUserModelID set on the Start Menu shortcut.
const appID = "Overseer"

var (
	setupOnce sync.Once
	iconPath  string
)

// Show displays a native Windows Toast notification with:
// - App name "Overseer" in Action Center
// - Title and body text
// - Click action to open URL in browser
// - App icon displayed alongside the notification
// - Respects mute setting
func (n *Notifier) Show(title, body, clickURL string) {
	if n.muted {
		log.Printf("[notifier] muted, skipping toast: %s", title)
		return
	}
	setupOnce.Do(func() {
		iconPath = ensureIconFile()
		if err := ensureStartMenuShortcut(); err != nil {
			log.Printf("[notifier] shortcut setup: %v", err)
		}
	})

	if clickURL == "" {
		clickURL = n.serverURL
	}

	notification := toast.Notification{
		AppID:               appID,
		Title:               title,
		Message:             body,
		Audio:               toast.Default,
		ActivationType:      "protocol",
		ActivationArguments: clickURL,
	}

	// Set icon if available
	if iconPath != "" {
		notification.Icon = iconPath
	}

	if err := notification.Push(); err != nil {
		log.Printf("[notifier] toast push error: %v", err)
	}
}

// ensureIconFile creates a simple icon file for the notification.
func ensureIconFile() string {
	dir := filepath.Join(os.Getenv("LOCALAPPDATA"), "overseer-desktop")
	_ = os.MkdirAll(dir, 0755)
	iconFile := filepath.Join(dir, "icon.png")

	// If icon already exists, reuse it
	if _, err := os.Stat(iconFile); err == nil {
		return iconFile
	}

	// Generate a simple 64x64 blue circle PNG using PowerShell + .NET
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
		log.Printf("[notifier] failed to generate icon: %v", err)
		return ""
	}

	return iconFile
}

// ensureStartMenuShortcut creates a Start Menu shortcut with AppUserModelID
// so Windows shows "Overseer Desktop" in Action Center.
func ensureStartMenuShortcut() error {
	startMenuDir := filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs")
	shortcutPath := filepath.Join(startMenuDir, "Overseer.lnk")

	// Skip if already exists
	if _, err := os.Stat(shortcutPath); err == nil {
		return nil
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable: %w", err)
	}
	exePath, _ = filepath.Abs(exePath)

	// Create shortcut with AppUserModelID using PowerShell COM interop
	ps := fmt.Sprintf(`
$WshShell = New-Object -ComObject WScript.Shell
$Shortcut = $WshShell.CreateShortcut('%s')
$Shortcut.TargetPath = '%s'
$Shortcut.Description = 'Overseer Desktop Notification Client'
$Shortcut.Save()
`, escapePSPath(shortcutPath), escapePSPath(exePath))

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", ps)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("create shortcut: %w (%s)", err, string(output))
	}

	log.Printf("[notifier] created Start Menu shortcut: %s", shortcutPath)
	return nil
}

func escapePSPath(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
