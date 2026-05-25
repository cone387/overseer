package migrations

// InitialMigration contains the SQL statements for the initial schema.
var InitialMigration = []string{
	// Messages table
	`CREATE TABLE IF NOT EXISTS messages (
		id          TEXT PRIMARY KEY,
		source      TEXT NOT NULL,
		channel     TEXT NOT NULL,
		title       TEXT NOT NULL,
		body        TEXT NOT NULL,
		extra       TEXT,
		status      TEXT NOT NULL DEFAULT 'pending',
		fail_reason TEXT,
		retry_count INTEGER DEFAULT 0,
		received_at DATETIME NOT NULL,
		pushed_at   DATETIME,
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_messages_channel ON messages(channel)`,
	`CREATE INDEX IF NOT EXISTS idx_messages_status ON messages(status)`,
	`CREATE INDEX IF NOT EXISTS idx_messages_received_at ON messages(received_at)`,

	// Reminders table
	`CREATE TABLE IF NOT EXISTS reminders (
		id             TEXT PRIMARY KEY,
		title          TEXT NOT NULL,
		body           TEXT,
		channel        TEXT NOT NULL DEFAULT 'default',
		trigger_at     DATETIME NOT NULL,
		repeat_type    TEXT NOT NULL DEFAULT 'once',
		repeat_rule    TEXT,
		status         TEXT NOT NULL DEFAULT 'active',
		next_trigger   DATETIME,
		last_triggered DATETIME,
		fail_reason    TEXT,
		created_at     DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at     DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_reminders_status ON reminders(status)`,
	`CREATE INDEX IF NOT EXISTS idx_reminders_next_trigger ON reminders(next_trigger)`,

	// Push results table
	`CREATE TABLE IF NOT EXISTS push_results (
		id          TEXT PRIMARY KEY,
		message_id  TEXT NOT NULL REFERENCES messages(id),
		device_key  TEXT NOT NULL,
		status      TEXT NOT NULL DEFAULT 'pending',
		fail_reason TEXT,
		retry_count INTEGER DEFAULT 0,
		pushed_at   DATETIME,
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_push_results_message ON push_results(message_id)`,
}
