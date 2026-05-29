package model

import "time"

// Channel represents a push notification channel stored in the database.
type Channel struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Sound          string    `json:"sound"`
	Group          string    `json:"group"`
	Icon           string    `json:"icon"`
	Level          string    `json:"level"`
	DeviceKeys     []string  `json:"device_keys"`
	RequireAck     bool      `json:"require_ack"`
	RepeatInterval string    `json:"repeat_interval"`
	MaxRepeats     int       `json:"max_repeats"`
	SoundFile      string    `json:"sound_file"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
