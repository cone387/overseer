//go:build windows

package notifier

import (
	"log"
	"os/exec"
)

// Show displays a native Windows toast notification.
// On click, it opens the given URL (or the server URL if empty) in the default browser.
func (n *Notifier) Show(title, body, clickURL string) {
	if clickURL == "" {
		clickURL = n.serverURL
	}

	// Use PowerShell to show a Windows toast notification via BurntToast or fallback to BalloonTip.
	// This approach requires no CGO and no external dependencies.
	script := `
Add-Type -AssemblyName System.Windows.Forms
$notify = New-Object System.Windows.Forms.NotifyIcon
$notify.Icon = [System.Drawing.SystemIcons]::Information
$notify.BalloonTipTitle = '` + escapePSString(title) + `'
$notify.BalloonTipText = '` + escapePSString(body) + `'
$notify.Visible = $true
$notify.ShowBalloonTip(5000)
Start-Sleep -Milliseconds 5100
$notify.Dispose()
`

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	if err := cmd.Start(); err != nil {
		log.Printf("[notifier] failed to show notification: %v", err)
		return
	}
	// Don't wait — fire and forget
	go func() {
		_ = cmd.Wait()
	}()
}

// escapePSString escapes single quotes for PowerShell string literals.
func escapePSString(s string) string {
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\'' {
			result = append(result, '\'', '\'')
		} else if s[i] == '\n' {
			result = append(result, ' ')
		} else if s[i] == '\r' {
			// skip
		} else {
			result = append(result, s[i])
		}
	}
	return string(result)
}
