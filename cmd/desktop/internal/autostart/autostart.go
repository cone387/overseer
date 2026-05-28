package autostart

// IsEnabled returns whether auto-start is currently registered.
func IsEnabled() bool {
	return isEnabled()
}

// Enable registers the application for auto-start on boot.
func Enable() error {
	return enable()
}

// Disable removes the application from auto-start.
func Disable() error {
	return disable()
}
