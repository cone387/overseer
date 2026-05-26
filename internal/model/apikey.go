package model

import "time"

// APIKey represents a user-generated API key for programmatic access.
type APIKey struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`      // user-friendly name like "GitHub Webhook"
	Key       string     `json:"key"`       // the actual key (shown only on creation)
	KeyHash   string     `json:"-"`         // bcrypt hash for verification
	Prefix    string     `json:"prefix"`    // first 8 chars for display
	UserID    string     `json:"user_id"`
	CreatedAt time.Time  `json:"created_at"`
	LastUsed  *time.Time `json:"last_used,omitempty"`
}
