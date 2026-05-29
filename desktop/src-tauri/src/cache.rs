use chrono::{DateTime, Utc};
use rusqlite::{params, Connection};
use serde::{Deserialize, Serialize};
use std::path::PathBuf;
use std::sync::Mutex;

const MAX_ENTRIES: u32 = 50;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct NotificationEntry {
    pub id: String,
    pub title: String,
    pub body: String,
    pub url: String,
    pub channel: String,
    pub source: String,
    pub level: String,
    pub received_at: String,
    pub unread: bool,
    pub acked_at: Option<String>,
    pub expires_at: Option<String>,
}

pub struct Cache {
    conn: Mutex<Connection>,
}

impl Cache {
    pub fn new() -> Result<Self, String> {
        let db_path = Self::db_path();
        let dir = db_path.parent().unwrap();
        std::fs::create_dir_all(dir).map_err(|e| format!("create cache dir: {}", e))?;

        let conn =
            Connection::open(&db_path).map_err(|e| format!("open cache db: {}", e))?;

        conn.execute_batch(
            "CREATE TABLE IF NOT EXISTS notifications (
                id TEXT PRIMARY KEY,
                title TEXT NOT NULL,
                body TEXT DEFAULT '',
                url TEXT DEFAULT '',
                channel TEXT DEFAULT '',
                source TEXT DEFAULT '',
                level TEXT DEFAULT 'default',
                received_at TEXT NOT NULL,
                unread INTEGER DEFAULT 1,
                acked_at TEXT,
                expires_at TEXT
            );",
        )
        .map_err(|e| format!("create table: {}", e))?;

        // Migrate: add level column if it doesn't exist (for databases from Go version)
        let _ = conn.execute_batch("ALTER TABLE notifications ADD COLUMN level TEXT DEFAULT 'default'");

        Ok(Self {
            conn: Mutex::new(conn),
        })
    }

    fn db_path() -> PathBuf {
        let base = dirs::data_local_dir().unwrap_or_else(|| PathBuf::from("."));
        base.join("overseer-desktop").join("notifications.db")
    }

    pub fn add(&self, entry: &NotificationEntry) -> Result<(), String> {
        let conn = self.conn.lock().unwrap();
        conn.execute(
            "INSERT OR REPLACE INTO notifications (id, title, body, url, channel, source, level, received_at, unread, acked_at, expires_at) VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11)",
            params![
                entry.id,
                entry.title,
                entry.body,
                entry.url,
                entry.channel,
                entry.source,
                entry.level,
                entry.received_at,
                entry.unread as i32,
                entry.acked_at,
                entry.expires_at,
            ],
        )
        .map_err(|e| format!("insert notification: {}", e))?;

        // Enforce max entries
        conn.execute(
            "DELETE FROM notifications WHERE id NOT IN (SELECT id FROM notifications ORDER BY received_at DESC LIMIT ?1)",
            params![MAX_ENTRIES],
        )
        .map_err(|e| format!("enforce max entries: {}", e))?;

        Ok(())
    }

    pub fn mark_read(&self, id: &str) -> Result<(), String> {
        let conn = self.conn.lock().unwrap();
        conn.execute(
            "UPDATE notifications SET unread = 0 WHERE id = ?1",
            params![id],
        )
        .map_err(|e| format!("mark read: {}", e))?;
        Ok(())
    }

    pub fn mark_all_read(&self) -> Result<(), String> {
        let conn = self.conn.lock().unwrap();
        conn.execute("UPDATE notifications SET unread = 0 WHERE unread = 1", [])
            .map_err(|e| format!("mark all read: {}", e))?;
        Ok(())
    }

    pub fn mark_ack_synced(&self, id: &str) -> Result<(), String> {
        let conn = self.conn.lock().unwrap();
        let now = Utc::now().to_rfc3339();
        conn.execute(
            "UPDATE notifications SET unread = 0, acked_at = ?1 WHERE id = ?2",
            params![now, id],
        )
        .map_err(|e| format!("mark ack synced: {}", e))?;
        Ok(())
    }

    pub fn unread_count(&self) -> Result<i32, String> {
        let conn = self.conn.lock().unwrap();
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

    pub fn recent(&self, limit: u32) -> Result<Vec<NotificationEntry>, String> {
        let conn = self.conn.lock().unwrap();
        let mut stmt = conn
            .prepare(
                "SELECT id, title, body, url, channel, source, level, received_at, unread, acked_at, expires_at FROM notifications ORDER BY received_at DESC LIMIT ?1",
            )
            .map_err(|e| format!("prepare recent: {}", e))?;

        let entries = stmt
            .query_map(params![limit], |row| {
                Ok(NotificationEntry {
                    id: row.get(0)?,
                    title: row.get(1)?,
                    body: row.get::<_, String>(2).unwrap_or_default(),
                    url: row.get::<_, String>(3).unwrap_or_default(),
                    channel: row.get::<_, String>(4).unwrap_or_default(),
                    source: row.get::<_, String>(5).unwrap_or_default(),
                    level: row.get::<_, String>(6).unwrap_or_default(),
                    received_at: row.get(7)?,
                    unread: row.get::<_, i32>(8).unwrap_or(0) == 1,
                    acked_at: row.get(9).ok(),
                    expires_at: row.get(10).ok(),
                })
            })
            .map_err(|e| format!("query recent: {}", e))?
            .filter_map(|r| r.ok())
            .collect();

        Ok(entries)
    }

    pub fn get_unexpired_unread(&self) -> Result<Vec<NotificationEntry>, String> {
        let conn = self.conn.lock().unwrap();
        let now = Utc::now().to_rfc3339();
        let mut stmt = conn
            .prepare(
                "SELECT id, title, body, url, channel, source, level, received_at, unread, acked_at, expires_at FROM notifications WHERE unread = 1 AND (expires_at IS NULL OR expires_at > ?1) ORDER BY received_at DESC",
            )
            .map_err(|e| format!("prepare unread: {}", e))?;

        let entries = stmt
            .query_map(params![now], |row| {
                Ok(NotificationEntry {
                    id: row.get(0)?,
                    title: row.get(1)?,
                    body: row.get::<_, String>(2).unwrap_or_default(),
                    url: row.get::<_, String>(3).unwrap_or_default(),
                    channel: row.get::<_, String>(4).unwrap_or_default(),
                    source: row.get::<_, String>(5).unwrap_or_default(),
                    level: row.get::<_, String>(6).unwrap_or_default(),
                    received_at: row.get(7)?,
                    unread: true,
                    acked_at: row.get(9).ok(),
                    expires_at: row.get(10).ok(),
                })
            })
            .map_err(|e| format!("query unread: {}", e))?
            .filter_map(|r| r.ok())
            .collect();

        Ok(entries)
    }

    pub fn is_expired(expires_at: &Option<String>) -> bool {
        if let Some(exp) = expires_at {
            if let Ok(exp_time) = DateTime::parse_from_rfc3339(exp) {
                return Utc::now() > exp_time;
            }
        }
        false
    }
}
