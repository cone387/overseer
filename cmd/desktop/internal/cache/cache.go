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
	ID        string
	Title     string
	Body      string
	URL       string
	Channel   string
	Source    string
	ReceivedAt time.Time
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
		received_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("create table: %w", err)
	}

	return &Cache{db: db}, nil
}

// Add stores a notification entry, enforcing the max limit.
func (c *Cache) Add(e Entry) error {
	_, err := c.db.Exec(`INSERT OR REPLACE INTO notifications (id, title, body, url, channel, source, received_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.Title, e.Body, e.URL, e.Channel, e.Source, e.ReceivedAt)
	if err != nil {
		return err
	}

	// Enforce max entries
	_, err = c.db.Exec(`DELETE FROM notifications WHERE id NOT IN (SELECT id FROM notifications ORDER BY received_at DESC LIMIT ?)`, maxEntries)
	return err
}

// Recent returns the N most recent notifications.
func (c *Cache) Recent(n int) ([]Entry, error) {
	rows, err := c.db.Query(`SELECT id, title, body, url, channel, source, received_at FROM notifications ORDER BY received_at DESC LIMIT ?`, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var e Entry
		var body, url, channel, source sql.NullString
		if err := rows.Scan(&e.ID, &e.Title, &body, &url, &channel, &source, &e.ReceivedAt); err != nil {
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
		entries = append(entries, e)
	}
	return entries, nil
}

// Close closes the database connection.
func (c *Cache) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
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
