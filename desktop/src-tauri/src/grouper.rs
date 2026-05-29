use crate::websocket::PushEvent;
use log::info;
use std::collections::HashMap;
use std::sync::{Arc, Mutex};
use std::time::Duration;
use tauri::{AppHandle, Emitter};
use tokio::time::Instant;

const GROUP_WINDOW: Duration = Duration::from_secs(30);

#[derive(Debug, Clone)]
struct PendingGroup {
    source: String,
    events: Vec<PushEvent>,
    first_at: Instant,
}

/// Groups rapid notifications from the same source within a 30-second window.
/// The first notification is shown immediately; subsequent ones from the same source
/// within the window are batched into a summary notification.
pub struct Grouper {
    pending: Arc<Mutex<HashMap<String, PendingGroup>>>,
    app: AppHandle,
}

impl Grouper {
    pub fn new(app: AppHandle) -> Self {
        let pending: Arc<Mutex<HashMap<String, PendingGroup>>> =
            Arc::new(Mutex::new(HashMap::new()));

        // Spawn flush loop
        let pending_clone = pending.clone();
        let app_clone = app.clone();
        tauri::async_runtime::spawn(async move {
            loop {
                tokio::time::sleep(Duration::from_secs(5)).await;
                Self::flush_expired(&pending_clone, &app_clone);
            }
        });

        Self { pending, app }
    }

    /// Process an incoming push event.
    /// Returns Some(event) if it should be shown immediately, None if grouped.
    pub fn ingest(&self, event: PushEvent) -> Option<PushEvent> {
        let source = event.source.clone();
        if source.is_empty() {
            // No source — show immediately without grouping
            return Some(event);
        }

        let mut pending = self.pending.lock().unwrap();

        if let Some(group) = pending.get_mut(&source) {
            // Add to existing group
            group.events.push(event);
            info!(
                "[grouper] batched notification from '{}' (count: {})",
                source,
                group.events.len()
            );
            None
        } else {
            // First event from this source — show immediately and start group
            let group = PendingGroup {
                source: source.clone(),
                events: vec![event.clone()],
                first_at: Instant::now(),
            };
            pending.insert(source, group);
            Some(event)
        }
    }

    fn flush_expired(
        pending: &Arc<Mutex<HashMap<String, PendingGroup>>>,
        app: &AppHandle,
    ) {
        let now = Instant::now();
        let mut to_flush = Vec::new();

        {
            let mut map = pending.lock().unwrap();
            let expired_keys: Vec<String> = map
                .iter()
                .filter(|(_, group)| now.duration_since(group.first_at) >= GROUP_WINDOW)
                .map(|(key, _)| key.clone())
                .collect();

            for key in expired_keys {
                if let Some(group) = map.remove(&key) {
                    to_flush.push(group);
                }
            }
        }

        for group in to_flush {
            if group.events.len() > 1 {
                // Show summary for events 2+ (first was already shown)
                let count = group.events.len() - 1;
                let summary_title = format!("[{}] {} 条新通知", group.source, count);
                let summary_body = group.events.last().map(|e| e.title.clone()).unwrap_or_default();

                let summary = PushEvent {
                    id: String::new(),
                    source: group.source,
                    channel: group.events[0].channel.clone(),
                    title: summary_title,
                    body: summary_body,
                    url: String::new(),
                    icon: String::new(),
                    level: "default".to_string(),
                    status: String::new(),
                    time: String::new(),
                    expires_at: String::new(),
                };

                info!("[grouper] flushing summary for '{}'", summary.source);
                let _ = app.emit("show-notification", &summary);
            }
            // If only 1 event, it was already shown immediately
        }
    }
}
