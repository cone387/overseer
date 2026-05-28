//go:build darwin

package notifier

import (
	"log"
	"os/exec"
	"strings"
)

// Show displays a native macOS notification using osascript.
func (n *Notifier) Show(title, body, clickURL string) {
	if n.muted {
		log.Printf("[notifier] muted, skipping: %s", title)
		return
	}

	title = escapeAS(title)
	body = escapeAS(body)

	script := `display notification "` + body + `" with title "` + title + `"`

	cmd := exec.Command("osascript", "-e", script)
	if err := cmd.Start(); err != nil {
		log.Printf("[notifier] failed to show notification: %v", err)
		return
	}
	go func() { _ = cmd.Wait() }()
}

func escapeAS(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}
