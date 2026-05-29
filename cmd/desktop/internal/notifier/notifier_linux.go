//go:build linux

package notifier

import (
	"log"
	"os/exec"
)

// Show displays a basic Linux notification.
func (n *Notifier) Show(title, body, clickURL string) {
	n.showWithLevel(title, body, clickURL, "default", "")
}

func (n *Notifier) showWithLevel(title, body, clickURL, level, msgID string) {
	n.mu.Lock()
	muted := n.muted
	n.mu.Unlock()

	if muted {
		log.Printf("[notifier] muted, skipping: %s", title)
		return
	}

	urgency := "normal"
	switch level {
	case "critical":
		urgency = "critical"
	case "passive":
		urgency = "low"
	}

	cmd := exec.Command("notify-send", "-u", urgency, title, body)
	if err := cmd.Start(); err != nil {
		log.Printf("[notifier] failed: %v", err)
		return
	}
	go func() { _ = cmd.Wait() }()
}
