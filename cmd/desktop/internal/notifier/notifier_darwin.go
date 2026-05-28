//go:build darwin

package notifier

import (
	"log"
	"os/exec"
	"strings"
)

// Show displays a basic macOS notification.
func (n *Notifier) Show(title, body, clickURL string) {
	n.showWithLevel(title, body, clickURL, "default")
}

func (n *Notifier) showWithLevel(title, body, clickURL, level string) {
	n.mu.Lock()
	muted := n.muted
	n.mu.Unlock()

	if muted {
		log.Printf("[notifier] muted, skipping: %s", title)
		return
	}

	t := escapeAS(title)
	b := escapeAS(body)
	script := `display notification "` + b + `" with title "` + t + `"`

	cmd := exec.Command("osascript", "-e", script)
	if err := cmd.Start(); err != nil {
		log.Printf("[notifier] failed: %v", err)
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
