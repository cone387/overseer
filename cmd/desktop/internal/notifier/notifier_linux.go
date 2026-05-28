//go:build linux

package notifier

import (
	"log"
	"os/exec"
)

// Show displays a notification using notify-send (available on most Linux desktops).
func (n *Notifier) Show(title, body, clickURL string) {
	cmd := exec.Command("notify-send", title, body)
	if err := cmd.Start(); err != nil {
		log.Printf("[notifier] failed to show notification: %v", err)
		return
	}
	go func() {
		_ = cmd.Wait()
	}()
}
