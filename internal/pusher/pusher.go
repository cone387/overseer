package pusher

import "context"

// Pusher is the abstract interface for push channels, allowing future
// extension to FCM/APNs and other providers.
type Pusher interface {
	Push(ctx context.Context, req PushRequest) (*PushResponse, error)
	Name() string
}

// PushRequest contains all parameters needed to send a push notification.
type PushRequest struct {
	DeviceKey string
	Title     string
	Body      string
	Sound     string
	Group     string
	Icon      string
	Level     string
	URL       string
	Badge     int
}

// PushResponse contains the result of a push attempt.
type PushResponse struct {
	Success   bool
	MessageID string
	Error     string
}
