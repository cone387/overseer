package migrations

// SettingsMigration adds a key-value settings table for runtime configuration.
var SettingsMigration = []string{
	`CREATE TABLE IF NOT EXISTS settings (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL
	)`,
}
