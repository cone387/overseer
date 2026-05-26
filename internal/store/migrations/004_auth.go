package migrations

// AuthMigration creates the users and api_keys tables.
var AuthMigration = []string{
	`CREATE TABLE IF NOT EXISTS users (
		id            TEXT PRIMARY KEY,
		username      TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at    DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS api_keys (
		id         TEXT PRIMARY KEY,
		name       TEXT NOT NULL,
		key_hash   TEXT NOT NULL,
		prefix     TEXT NOT NULL,
		user_id    TEXT NOT NULL REFERENCES users(id),
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_used  DATETIME
	)`,
}
