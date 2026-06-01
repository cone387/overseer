use chrono::{DateTime, Utc};
use rusqlite::{params, Connection};
use serde::{Deserialize, Serialize};
use std::path::PathBuf;
use std::sync::Mutex;

const MAX_ENTRIES: i64 = 50;

/// A cached notification entry.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Entry {
    pub id: String,
    pub title: String,
    pub body: String,
    pub url: String,
    pub channel: String,
    pub source: String,
    pub received_at: DateTime<Utc>,
    pub unread: bool,
    pub acked_at: Option<DateTime<Utc>>,
    pub expires_at: Option<DateTime<Utc>>,
}

/// Notification cache backed by SQLite.
pub struct Cache {
    conn: Mutex<Connection>,
}

impl Cache {
    /// Create a new cache, initializing the SQLite database.
    pub fn new() -> Result<Self, String> {
        let db_path = Self::db_file_path()?;
        if let Some(dir) = db_path.parent() {
            std::fs::create_dir_all(dir).map_err(|e| format!("create cache dir: {}", e))?;
        }

        let conn =
            Connection::open(&db_path).map_err(|e| format!("open cache db: {}", e))?;

        conn.execute_batch(
            "CREATE TABLE IF NOT EXISTS notifications (
                id TEXT PRIMARY KEY,
                title TEXT NOT NULL,
                body TEXT,
                url TEXT,
                channel TEXT,
                source TEXT,
                received_at TEXT DEFAULT (datetime('now')),
                unread INTEGER DEFAULT 1,
                acked_at TEXT,
                expires_at TEXT
            );",
        )
        .map_err(|e| format!("create table: {}", e))?;

        Ok(Self {
            conn: Mutex::new(conn),
        })
    }

    /// Add a notification entry, enforcing the max limit.
    pub fn add(&self, entry: &Entry) -> Result<(), String> {
        let conn = self.conn.lock().map_err(|e| e.to_string())?;
        conn.execute(
            "INSERT OR REPLACE INTO notifications (id, title, body, url, channel, source, received_at, unread, acked_at, expires_at) VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10)",
            params![
                entry.id,
                entry.title,
                entry.body,
                entry.url,
                entry.channel,
                entry.source,
                entry.received_at.to_rfc3339(),
                entry.unread as i32,
                entry.acked_at.map(|t| t.to_rfc3339()),
                entry.expires_at.map(|t| t.to_rfc3339()),
            ],
        ).map_err(|e| format!("insert: {}", e))?;

        // Enforce max entries
        conn.execute(
            "DELETE FROM notifications WHERE id NOT IN (SELECT id FROM notifications ORDER BY received_at DESC LIMIT ?1)",
            params![MAX_ENTRIES],
        ).map_err(|e| format!("cleanup: {}", e))?;

        Ok(())
    }

    /// Mark a single notification as read.
    pub fn mark_read(&self, id: &str) -> Result<(), String> {
        let conn = self.conn.lock().map_err(|e| e.to_string())?;
        conn.execute("UPDATE notifications SET unread = 0 WHERE id = ?1", params![id])
            .map_err(|e| format!("mark read: {}", e))?;
        Ok(())
    }

    /// Mark all notifications as read.
    pub fn mark_all_read(&self) -> Result<(), String> {
        let conn = self.conn.lock().map_err(|e| e.to_string())?;
        conn.execute("UPDATE notifications SET unread = 0 WHERE unread = 1", [])
            .map_err(|e| format!("mark all read: {}", e))?;
        Ok(())
    }

    /// Get the count of unexpired unread notifications.
    pub fn unread_count(&self) -> Result<i32, String> {
        let conn = self.conn.lock().map_err(|e| e.to_string())?;
        let now = Utc::now().to_rfc3339();
        let count: i32 = conn
            .query_row(
                "SELECT COUNT(*) FROM notifications WHERE unread = 1 AND (expires_at IS NULL OR expires_at > ?1)",
                params![now],
                |row| row.get(0),
            )
            .map_err(|e| format!("unread count: {}", e))?;
        Ok(count)
    }

    /// Mark a notification as ack-synced (read + acked_at timestamp).
    pub fn mark_ack_synced(&self, id: &str) -> Result<(), String> {
        let conn = self.conn.lock().map_err(|e| e.to_string())?;
        let now = Utc::now().to_rfc3339();
        conn.execute(
            "UPDATE notifications SET unread = 0, acked_at = ?1 WHERE id = ?2",
            params![now, id],
        )
        .map_err(|e| format!("ack sync: {}", e))?;
        Ok(())
    }

    /// Get the N most recent notifications.
    pub fn recent(&self, n: i32) -> Result<Vec<Entry>, String> {
        let conn = self.conn.lock().map_err(|e| e.to_string())?;
        let mut stmt = conn
            .prepare("SELECT id, title, body, url, channel, source, received_at, unread, acked_at, expires_at FROM notifications ORDER BY received_at DESC LIMIT ?1")
            .map_err(|e| format!("prepare: {}", e))?;

        let entries = stmt
            .query_map(params![n], |row| {
                Ok(Self::row_to_entry(row))
            })
            .map_err(|e| format!("query: {}", e))?
            .filter_map(|r| r.ok())
            .collect();

        Ok(entries)
    }

    fn row_to_entry(row: &rusqlite::Row) -> Entry {
        let received_at_str: String = row.get(6).unwrap_or_default();
        let unread_int: i32 = row.get(7).unwrap_or(1);
        let acked_at_str: Option<String> = row.get(8).unwrap_or(None);
        let expires_at_str: Option<String> = row.get(9).unwrap_or(None);

        Entry {
            id: row.get(0).unwrap_or_default(),
            title: row.get(1).unwrap_or_default(),
            body: row.get(2).unwrap_or_default(),
            url: row.get(3).unwrap_or_default(),
            channel: row.get(4).unwrap_or_default(),
            source: row.get(5).unwrap_or_default(),
            received_at: DateTime::parse_from_rfc3339(&received_at_str)
                .map(|dt| dt.with_timezone(&Utc))
                .unwrap_or_else(|_| Utc::now()),
            unread: unread_int == 1,
            acked_at: acked_at_str
                .and_then(|s| DateTime::parse_from_rfc3339(&s).ok())
                .map(|dt| dt.with_timezone(&Utc)),
            expires_at: expires_at_str
                .and_then(|s| DateTime::parse_from_rfc3339(&s).ok())
                .map(|dt| dt.with_timezone(&Utc)),
        }
    }

    fn db_file_path() -> Result<PathBuf, String> {
        dirs::config_dir()
            .map(|d| d.join("overseer-desktop").join("notifications.db"))
            .ok_or_else(|| "cannot determine config dir".to_string())
    }
}
