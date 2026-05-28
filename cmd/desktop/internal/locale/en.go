package locale

var enStrings = map[string]string{
	// Tray menu
	"tray.connected":    "● Connected",
	"tray.disconnected": "○ Disconnected",
	"tray.mute":         "Mute",
	"tray.autostart":    "Start on Boot",
	"tray.settings":     "Settings",
	"tray.open_webui":   "Open Web UI",
	"tray.reconnect":    "Reconnect",
	"tray.quit":         "Quit",
	"tray.recent":       "Recent Notifications",
	"tray.view_all":     "View All",
	"tray.no_history":   "No notifications",

	// Setup dialog
	"setup.title":        "Overseer Desktop - Setup",
	"setup.welcome":      "Welcome to Overseer Desktop",
	"setup.server_url":   "Server URL",
	"setup.token":        "Registration Token",
	"setup.device_name":  "Device Name (optional)",
	"setup.connect":      "Connect",
	"setup.cancel":       "Cancel",
	"setup.validation":   "Please fill in server URL and registration token",

	// Settings dialog
	"settings.title":       "Overseer Desktop - Settings",
	"settings.server_url":  "Server URL",
	"settings.device_name": "Device Name",
	"settings.device_id":   "Device ID",
	"settings.re_register": "Re-register",
	"settings.reset":       "Reset Config",
	"settings.save":        "Save",
	"settings.cancel":      "Cancel",

	// Notifications
	"notify.update_title":   "Update Available",
	"notify.update_message": "Overseer Desktop %s is available. Click to download.",
	"notify.open":           "Open",

	// Status
	"status.muted":   "Notifications muted",
	"status.unmuted": "Notifications unmuted",
}
