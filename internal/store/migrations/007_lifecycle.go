package migrations

// LifecycleMigration contains statements that can use IF NOT EXISTS safely.
var LifecycleMigration = []string{
	`CREATE INDEX IF NOT EXISTS idx_messages_unacked ON messages(channel, ack_at, expires_at, status)`,
	`CREATE INDEX IF NOT EXISTS idx_messages_catchup ON messages(received_at, ack_at)`,
}

// LifecycleAlterMessagesSQL contains ALTER TABLE statements for the messages table.
// Must be executed conditionally (only if columns don't exist yet).
var LifecycleAlterMessagesSQL = []string{
	`ALTER TABLE messages ADD COLUMN ack_at DATETIME`,
	`ALTER TABLE messages ADD COLUMN snooze_until DATETIME`,
	`ALTER TABLE messages ADD COLUMN expires_at DATETIME`,
	`ALTER TABLE messages ADD COLUMN repeat_count INTEGER DEFAULT 0`,
}

// LifecycleAlterDevicesSQL is the ALTER TABLE statement to add last_seen to devices.
// Must be executed conditionally (only if column doesn't exist yet).
const LifecycleAlterDevicesSQL = `ALTER TABLE devices ADD COLUMN last_seen DATETIME`
