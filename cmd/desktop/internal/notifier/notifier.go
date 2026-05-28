package notifier

import "sync"

// Notifier displays native OS notifications.
type Notifier struct {
	serverURL string
	muted     bool
	mu        sync.Mutex
}

// New creates a new Notifier that can open the Web UI at the given server URL.
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
