package migrations

// DevicesMigration adds the devices table for managing push target devices.
var DevicesMigration = []string{
	`CREATE TABLE IF NOT EXISTS devices (
		id         TEXT PRIMARY KEY,
		name       TEXT NOT NULL,
		device_key TEXT NOT NULL UNIQUE,
		is_default INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_devices_device_key ON devices(device_key)`,
}
