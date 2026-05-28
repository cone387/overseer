package model

import "time"

// Device type constants.
const (
	DeviceTypeBark    = "bark"
	DeviceTypeDesktop = "desktop"
)

// Device represents a registered push target device.
type Device struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`       // user-friendly alias, e.g. "我的 iPhone"
	DeviceKey string    `json:"device_key"` // Bark device key or desktop API key
	Type      string    `json:"type"`       // "bark" | "desktop"
	IsDefault bool      `json:"is_default"` // whether this is the default push target
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
