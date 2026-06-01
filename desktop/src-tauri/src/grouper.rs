use log::info;
use std::collections::HashMap;
use std::sync::{Arc, Mutex};
use tokio::time::{sleep, Duration};

/// Push event for grouping.
#[derive(Debug, Clone)]
pub struct PushEvent {
    pub id: String,
    pub source: String,
    pub channel: String,
    pub title: String,
    pub body: String,
    pub url: String,
    pub level: String,
}

/// Callback type for showing a notification.
pub type ShowFn = Arc<dyn Fn(String, String, String, String, String, String, String) + Send + Sync>;

struct PendingGroup {
    events: Vec<PushEvent>,
}

/// Groups rapid notifications from the same source within a 30-second window.
pub struct Grouper {
    window: Duration,
    pending: Arc<Mutex<HashMap<String, PendingGroup>>>,
    show_fn: ShowFn,
}

impl Grouper {
    pub fn new(show_fn: ShowFn) -> Self {
        Self {
            window: Duration::from_secs(30),
            pending: Arc::new(Mutex::new(HashMap::new())),
            show_fn,
        }
    }

    /// Process an incoming push event.
    pub fn ingest(&self, event: PushEvent) {
        let mut pending = self.pending.lock().unwrap();

        if let Some(group) = pending.get_mut(&event.source) {
            // Add to existing group - already shown the first one
            group.events.push(event);
            return;
        }

        // First event from this source - show immediately
        (self.show_fn)(
            event.title.clone(),
            event.body.clone(),
            event.url.clone(),
            event.channel.clone(),
            event.source.clone(),
            event.level.clone(),
            event.id.clone(),
        );

        // Start a new group
        let source = event.source.clone();
        pending.insert(
            source.clone(),
            PendingGroup {
                events: vec![event],
            },
        );
        drop(pending);

        // Schedule flush after window expires
        let pending_ref = self.pending.clone();
        let show_fn = self.show_fn.clone();
        let window = self.window;

        tokio::spawn(async move {
            sleep(window).await;
            flush(&source, &pending_ref, &show_fn);
        });
    }
}

fn flush(
    source: &str,
    pending: &Arc<Mutex<HashMap<String, PendingGroup>>>,
    show_fn: &ShowFn,
) {
    let group = {
        let mut map = pending.lock().unwrap();
        map.remove(source)
    };

    let Some(group) = group else { return };

    // If only 1 event, it was already shown immediately
    if group.events.len() <= 1 {
        return;
    }

    // Show summary for events 2+ (first was already shown)
    let count = group.events.len() - 1;
    let title = format!("[{}] {} 条新通知", source, count);
    let body = group.events.last().map(|e| e.title.clone()).unwrap_or_default();
    let channel = group.events[0].channel.clone();

    info!("[grouper] flushing {} grouped notifications from {}", count, source);

    (show_fn)(
        title,
        body,
        String::new(),
        channel,
        source.to_string(),
        "default".to_string(),
        String::new(),
    );
}
