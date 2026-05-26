package migrations

// ChannelsMigration adds the channels table for web-managed channel configuration.
var ChannelsMigration = []string{
	`CREATE TABLE IF NOT EXISTS channels (
		id          TEXT PRIMARY KEY,
		name        TEXT NOT NULL UNIQUE,
		sound       TEXT NOT NULL DEFAULT '',
		grp         TEXT NOT NULL DEFAULT '',
		icon        TEXT NOT NULL DEFAULT '',
		level       TEXT NOT NULL DEFAULT 'active',
		device_keys TEXT NOT NULL DEFAULT '[]',
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_channels_name ON channels(name)`,
}
