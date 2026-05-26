package model

import "time"

// PushStatus represents the delivery status of a message.
type PushStatus string

const (
	StatusPending PushStatus = "pending"
	StatusSuccess PushStatus = "success"
	StatusFailed  PushStatus = "failed"
)

// Message represents an internal push message flowing through the system.
type Message struct {
	ID         string            `json:"id"`
	Source     string            `json:"source"`
	Channel    string            `json:"channel"`
	Title      string            `json:"title"`
	Body       string            `json:"body"`
	Extra      map[string]string `json:"extra,omitempty"`
	Status     PushStatus        `json:"status"`
	FailReason string            `json:"fail_reason,omitempty"`
	RetryCount int               `json:"retry_count"`
	ReceivedAt time.Time         `json:"received_at"`
	PushedAt   *time.Time        `json:"pushed_at,omitempty"`

	// Push parameter overrides (optional, override channel defaults)
	Sound string `json:"sound,omitempty"`
	Icon  string `json:"icon,omitempty"`
	Group string `json:"group,omitempty"`
	Level string `json:"level,omitempty"`
	URL   string `json:"url,omitempty"`
}
