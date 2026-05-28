package notifier

// Notifier displays native OS notifications.
type Notifier struct {
	serverURL string
}

// New creates a new Notifier that can open the Web UI at the given server URL.
func New(serverURL string) *Notifier {
	return &Notifier{serverURL: serverURL}
}
