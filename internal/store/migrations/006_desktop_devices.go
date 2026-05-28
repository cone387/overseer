package migrations

// DesktopDevicesMigration is intentionally empty because the ALTER TABLE
// must be run conditionally (SQLite doesn't support IF NOT EXISTS for ADD COLUMN).
// The actual migration logic is in DesktopDevicesMigrationFn.
var DesktopDevicesMigration = []string{
	`CREATE INDEX IF NOT EXISTS idx_devices_type_key ON devices(type, device_key)`,
}

// DesktopDevicesAlterSQL is the ALTER TABLE statement to add the type column.
// Must be executed conditionally (only if column doesn't exist yet).
const DesktopDevicesAlterSQL = `ALTER TABLE devices ADD COLUMN type TEXT NOT NULL DEFAULT 'bark'`
