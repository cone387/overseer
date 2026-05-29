use futures_util::{SinkExt, StreamExt};
use log::{info, warn, error};
use serde::Deserialize;
use std::sync::Arc;
use tokio::sync::{mpsc, Mutex, Notify};
use tokio::time::{sleep, Duration};
use tokio_tungstenite::{connect_async, tungstenite::Message};
use url::Url;

/// Push notification event received via WebSocket.
#[derive(Debug, Clone, Deserialize)]
pub struct PushEvent {
    pub id: String,
    #[serde(default)]
    pub source: String,
    #[serde(default)]
    pub channel: String,
    #[serde(default)]
    pub title: String,
    #[serde(default)]
    pub body: String,
    #[serde(default)]
    pub url: String,
    #[serde(default)]
    pub level: String,
    #[serde(default)]
    pub expires_at: String,
}

/// WebSocket event envelope.
#[derive(Debug, Deserialize)]
struct WsEvent {
    r#type: String,
    payload: serde_json::Value,
}

/// Events emitted by the WebSocket client.
#[derive(Debug, Clone)]
pub enum WsClientEvent {
    Push(PushEvent),
    AckSync { message_id: String },
    Connected,
    Disconnected,
    AuthFailed,
}

/// WebSocket client that connects to the Overseer server with auto-reconnect.
pub struct WsClient {
    server_url: String,
    api_key: String,
    event_tx: mpsc::UnboundedSender<WsClientEvent>,
    connected: Arc<Mutex<bool>>,
    shutdown: Arc<Notify>,
    reconnect_trigger: Arc<Notify>,
}

impl WsClient {
    pub fn new(
        server_url: String,
        api_key: String,
        event_tx: mpsc::UnboundedSender<WsClientEvent>,
    ) -> Self {
        Self {
            server_url,
            api_key,
            event_tx,
            connected: Arc::new(Mutex::new(false)),
            shutdown: Arc::new(Notify::new()),
            reconnect_trigger: Arc::new(Notify::new()),
        }
    }

    /// Start the WebSocket connection loop (runs until shutdown).
    pub async fn run(&self) {
        let mut backoff = Duration::from_secs(1);
        let max_backoff = Duration::from_secs(60);

        loop {
            // Check for shutdown before attempting connection
            tokio::select! {
                biased;
                _ = self.shutdown.notified() => return,
                result = self.connect_and_read() => {
                    match result {
                        Ok(()) => {
                            // Normal disconnection, reset backoff
                            backoff = Duration::from_secs(1);
                        }
                        Err(e) if e == "auth_failed" => {
                            let _ = self.event_tx.send(WsClientEvent::AuthFailed);
                            error!("[ws] auth failed, stopping");
                            return;
                        }
                        Err(e) => {
                            warn!("[ws] connection error: {} (retry in {:?})", e, backoff);
                        }
                    }
                }
            }

            // Wait for backoff, shutdown, or manual reconnect
            tokio::select! {
                _ = self.shutdown.notified() => return,
                _ = self.reconnect_trigger.notified() => {
                    backoff = Duration::from_secs(1);
                    continue;
                }
                _ = sleep(backoff) => {}
            }

            backoff = (backoff * 2).min(max_backoff);
        }
    }

    /// Force reconnection.
    pub fn reconnect(&self) {
        self.reconnect_trigger.notify_one();
    }

    /// Shutdown the client.
    pub fn close(&self) {
        self.shutdown.notify_one();
    }

    /// Connect to WebSocket and read messages until disconnection.
    async fn connect_and_read(&self) -> Result<(), String> {
        let mut url = Url::parse(&self.server_url).map_err(|e| e.to_string())?;

        // Convert http(s) to ws(s)
        match url.scheme() {
            "https" => url.set_scheme("wss").map_err(|_| "set scheme failed".to_string())?,
            _ => url.set_scheme("ws").map_err(|_| "set scheme failed".to_string())?,
        }
        url.set_path("/ws");
        url.query_pairs_mut().append_pair("api_key", &self.api_key);

        info!("[ws] connecting to {}...", url.as_str());

        let (ws_stream, _response) = connect_async(url.as_str())
            .await
            .map_err(|e| {
                let msg = e.to_string();
                if msg.contains("401") || msg.contains("Unauthorized") {
                    return "auth_failed".to_string();
                }
                msg
            })?;

        // Mark as connected
        {
            *self.connected.lock().await = true;
        }
        let _ = self.event_tx.send(WsClientEvent::Connected);
        info!("[ws] connected successfully");

        // Split into read/write halves
        let (mut write, mut read) = ws_stream.split();

        // Read loop - forward pings to write half for pong replies
        loop {
            match read.next().await {
                Some(Ok(msg)) => {
                    match msg {
                        Message::Text(text) => {
                            info!("[ws] received text: {} bytes", text.len());
                            self.handle_message(&text);
                        }
                        Message::Ping(data) => {
                            info!("[ws] received ping, sending pong");
                            let _ = write.send(Message::Pong(data)).await;
                        }
                        Message::Close(_) => {
                            info!("[ws] received close frame");
                            break;
                        }
                        _ => {}
                    }
                }
                Some(Err(e)) => {
                    warn!("[ws] read error: {}", e);
                    break;
                }
                None => {
                    info!("[ws] stream ended");
                    break;
                }
            }
        }

        *self.connected.lock().await = false;
        let _ = self.event_tx.send(WsClientEvent::Disconnected);
        Ok(())
    }

    fn handle_message(&self, text: &str) {
        let event: WsEvent = match serde_json::from_str(text) {
            Ok(e) => e,
            Err(e) => {
                warn!("[ws] parse error: {}", e);
                return;
            }
        };

        match event.r#type.as_str() {
            "push" => {
                if let Ok(push) = serde_json::from_value::<PushEvent>(event.payload) {
                    info!("[ws] push: [{}] {}", push.channel, push.title);
                    let _ = self.event_tx.send(WsClientEvent::Push(push));
                }
            }
            "ack_sync" => {
                if let Some(msg_id) = event.payload.get("message_id").and_then(|v| v.as_str()) {
                    let _ = self.event_tx.send(WsClientEvent::AckSync {
                        message_id: msg_id.to_string(),
                    });
                }
            }
            _ => {
                info!("[ws] unknown event type: {}", event.r#type);
            }
        }
    }
}
