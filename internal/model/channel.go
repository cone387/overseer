package model

import "time"

// Channel represents a push notification channel stored in the database.
type Channel struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Sound      string   `json:"sound"`
	Group      string   `json:"group"`
	Icon       string   `json:"icon"`
	Level      string   `json:"level"`
	DeviceKeys []string `json:"device_keys"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
