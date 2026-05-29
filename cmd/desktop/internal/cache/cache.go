package cache

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	_ "modernc.org/sqlite"
)

const maxEntries = 50

// Entry represents a cached notification.
type Entry struct {
	ID         string
	Title      string
	Body       string
	URL        string
	Channel    string
	Source     string
	ReceivedAt time.Time
	Unread     bool
	AckedAt    *time.Time
	ExpiresAt  *time.Time
}

// Cache manages local notification history in SQLite.
type Cache struct {
	db *sql.DB
}

// New creates a new Cache, initializing the SQLite database.
func New() (*Cache, error) {
	dbPath, err := dbFilePath()
	if err != nil {
		return nil, err
	}

	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create cache dir: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open cache db: %w", err)
	}

	// Create table
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS notifications (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		body TEXT,
		url TEXT,
		channel TEXT,
		source TEXT,
		received_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		unread INTEGER DEFAULT 1,
		acked_at DATETIME,
		expires_at DATETIME
	)`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("create table: %w", err)
	}

	// Migrate: add lifecycle columns if they don't exist (for existing databases)
	db.Exec(`ALTER TABLE notifications ADD COLUMN unread INTEGER DEFAULT 1`)
	db.Exec(`ALTER TABLE notifications ADD COLUMN acked_at DATETIME`)
	db.Exec(`ALTER TABLE notifications ADD COLUMN expires_at DATETIME`)

	return &Cache{db: db}, nil
}

// Add stores a notification entry, enforcing the max limit.
func (c *Cache) Add(e Entry) error {
	_, err := c.db.Exec(`INSERT OR REPLACE INTO notifications (id, title, body, url, channel, source, received_at, unread, acked_at, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.Title, e.Body, e.URL, e.Channel, e.Source, e.ReceivedAt, boolToInt(e.Unread), e.AckedAt, e.ExpiresAt)
	if err != nil {
		return err
	}

	// Enforce max entries
	_, err = c.db.Exec(`DELETE FROM notifications WHERE id NOT IN (SELECT id FROM notifications ORDER BY received_at DESC LIMIT ?)`, maxEntries)
	return err
}

// MarkRead marks a single notification as read.
func (c *Cache) MarkRead(id string) error {
	_, err := c.db.Exec(`UPDATE notifications SET unread = 0 WHERE id = ?`, id)
	return err
}

// MarkAllRead marks all unread notifications as read.
func (c *Cache) MarkAllRead() error {
	_, err := c.db.Exec(`UPDATE notifications SET unread = 0 WHERE unread = 1`)
	return err
}

// UnreadCount returns the number of unexpired unread notifications.
func (c *Cache) UnreadCount() (int, error) {
	var count int
	err := c.db.QueryRow(`SELECT COUNT(*) FROM notifications WHERE unread = 1 AND (expires_at IS NULL OR expires_at > ?)`, time.Now()).Scan(&count)
	return count, err
}

// MarkAckSynced marks a notification as read and records the ack timestamp.
func (c *Cache) MarkAckSynced(id string) error {
	now := time.Now()
	_, err := c.db.Exec(`UPDATE notifications SET unread = 0, acked_at = ? WHERE id = ?`, now, id)
	return err
}

// GetUnexpiredUnread returns all unread notifications that have not expired.
func (c *Cache) GetUnexpiredUnread() ([]Entry, error) {
	rows, err := c.db.Query(`SELECT id, title, body, url, channel, source, received_at, unread, acked_at, expires_at FROM notifications WHERE unread = 1 AND (expires_at IS NULL OR expires_at > ?) ORDER BY received_at DESC`, time.Now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanEntries(rows)
}

// Recent returns the N most recent notifications.
func (c *Cache) Recent(n int) ([]Entry, error) {
	rows, err := c.db.Query(`SELECT id, title, body, url, channel, source, received_at, unread, acked_at, expires_at FROM notifications ORDER BY received_at DESC LIMIT ?`, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanEntries(rows)
}

// Close closes the database connection.
func (c *Cache) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

func scanEntries(rows *sql.Rows) ([]Entry, error) {
	var entries []Entry
	for rows.Next() {
		var e Entry
		var body, url, channel, source sql.NullString
		var unread int
		var ackedAt, expiresAt sql.NullTime
		if err := rows.Scan(&e.ID, &e.Title, &body, &url, &channel, &source, &e.ReceivedAt, &unread, &ackedAt, &expiresAt); err != nil {
			return nil, err
		}
		if body.Valid {
			e.Body = body.String
		}
		if url.Valid {
			e.URL = url.String
		}
		if channel.Valid {
			e.Channel = channel.String
		}
		if source.Valid {
			e.Source = source.String
		}
		e.Unread = unread == 1
		if ackedAt.Valid {
			t := ackedAt.Time
			e.AckedAt = &t
		}
		if expiresAt.Valid {
			t := expiresAt.Time
			e.ExpiresAt = &t
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func dbFilePath() (string, error) {
	switch runtime.GOOS {
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			return "", fmt.Errorf("LOCALAPPDATA not set")
		}
		return filepath.Join(localAppData, "overseer-desktop", "notifications.db"), nil
	default:
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".config", "overseer-desktop", "notifications.db"), nil
	}
}
