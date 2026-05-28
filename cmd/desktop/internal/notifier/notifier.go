package notifier

import "sync"

// Notifier displays native OS notifications.
type Notifier struct {
	serverURL string
	muted     bool
	mu        sync.Mutex
}

// New creates a new Notifier.
func New(serverURL string) *Notifier {
	return &Notifier{serverURL: serverURL}
}

// SetMuted enables or disables notification muting.
func (n *Notifier) SetMuted(muted bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.muted = muted
}

// IsMuted returns whether notifications are currently muted.
func (n *Notifier) IsMuted() bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.muted
}

// ShowRich displays a notification with rich metadata (channel, source, level).
// Platform-specific implementations handle the actual display.
// Falls back to Show() on platforms that don't support rich notifications.
func (n *Notifier) ShowRich(title, body, clickURL, channel, source, level string) {
	// Build enriched body with source/channel info
	enrichedBody := body
	if source != "" && channel != "" {
		enrichedBody = "[" + channel + "] " + body
	} else if channel != "" {
		enrichedBody = "[" + channel + "] " + body
	}

	n.showWithLevel(title, enrichedBody, clickURL, level)
}
