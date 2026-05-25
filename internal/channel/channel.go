// Package channel provides a standalone channel lookup service.
// Other components (Aggregator, Scheduler, etc.) use it to resolve
// channel configurations by name with automatic fallback behavior.
package channel

import "github.com/overseer/overseer/internal/config"

// builtinDefault is the hard-coded fallback used when no "default" channel
// is defined in the user configuration.
var builtinDefault = config.Channel{
	Name:  "default",
	Sound: "",
	Group: "default",
	Level: "active",
}

// Manager holds channel configurations and provides lookup with fallback.
type Manager struct {
	channels       map[string]config.Channel
	defaultChannel config.Channel
}

// NewManager creates a Manager from the given channel list.
// It indexes channels by name and determines the effective default channel.
func NewManager(channels []config.Channel) *Manager {
	m := &Manager{
		channels: make(map[string]config.Channel, len(channels)),
	}

	for _, ch := range channels {
		m.channels[ch.Name] = ch
	}

	if ch, ok := m.channels["default"]; ok {
		m.defaultChannel = ch
	} else {
		m.defaultChannel = builtinDefault
	}

	return m
}

// GetChannel returns the channel configuration for the given name.
// If the name does not exist, it returns the "default" channel.
// If "default" is not configured, it returns the built-in default.
func (m *Manager) GetChannel(name string) config.Channel {
	if ch, ok := m.channels[name]; ok {
		return ch
	}
	return m.defaultChannel
}

// HasChannel reports whether a channel with the given name exists.
func (m *Manager) HasChannel(name string) bool {
	_, ok := m.channels[name]
	return ok
}

// All returns all configured channels.
func (m *Manager) All() []config.Channel {
	result := make([]config.Channel, 0, len(m.channels))
	for _, ch := range m.channels {
		result = append(result, ch)
	}
	return result
}
