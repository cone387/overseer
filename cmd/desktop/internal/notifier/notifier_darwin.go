//go:build darwin

package notifier

import (
	"log"
	"os/exec"
	"strings"
)

// Show displays a native macOS notification using osascript.
// Note: osascript notifications do not support click callbacks natively.
// The URL is logged but cannot be opened on click without a helper like terminal-notifier.
func (n *Notifier) Show(title, body, clickURL string) {
	// Escape for AppleScript string
	title = escapeAS(title)
	body = escapeAS(body)

	script := `display notification "` + body + `" with title "` + title + `"`

	cmd := exec.Command("osascript", "-e", script)
	if err := cmd.Start(); err != nil {
		log.Printf("[notifier] failed to show notification: %v", err)
		return
	}
	go func() {
		_ = cmd.Wait()
	}()
}

// escapeAS escapes characters for AppleScript string literals.
func escapeAS(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}
