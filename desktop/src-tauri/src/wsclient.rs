use futures_util::StreamExt;
use log::{error, info, warn};
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
    pub icon: String,
    #[serde(default)]
    pub level: String,
    #[serde(default)]
    pub status: String,
    #[serde(default)]
    pub time: String,
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

    pub async fn is_connected(&self) -> bool {
        *self.connected.lock().await
    }

    /// Start the WebSocket connection loop (runs until shutdown).
    pub async fn run(&self) {
        let mut backoff = Duration::from_secs(1);
        let max_backoff = Duration::from_secs(60);

        loop {
            match self.dial().await {
                Ok(()) => {
                    // Connected successfully, reset backoff
                    backoff = Duration::from_secs(1);
                    {
                        *self.connected.lock().await = true;
                    }
                    let _ = self.event_tx.send(WsClientEvent::Connected);
                    info!("[ws] connected");

                    // Read loop - blocks until disconnection
                    self.read_loop().await;

                    {
                        *self.connected.lock().await = false;
                    }
                    let _ = self.event_tx.send(WsClientEvent::Disconnected);
                    info!("[ws] disconnected");
                }
                Err(e) => {
                    if e == "auth_failed" {
                        let _ = self.event_tx.send(WsClientEvent::AuthFailed);
                        error!("[ws] auth failed, stopping reconnection");
                        return;
                    }
                    warn!("[ws] connection failed: {} (retry in {:?})", e, backoff);
                }
            }

            // Wait for backoff or shutdown/reconnect signal
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

    async fn dial(&self) -> Result<(), String> {
        let mut url = Url::parse(&self.server_url).map_err(|e| e.to_string())?;

        // Convert http(s) to ws(s)
        match url.scheme() {
            "https" => url.set_scheme("wss").map_err(|_| "set scheme failed")?,
            _ => url.set_scheme("ws").map_err(|_| "set scheme failed")?,
        }
        url.set_path("/ws");
        url.query_pairs_mut().append_pair("api_key", &self.api_key);

        let (ws_stream, response) = connect_async(url.as_str())
            .await
            .map_err(|e| {
                // Check for 401
                if e.to_string().contains("401") {
                    return "auth_failed".to_string();
                }
                e.to_string()
            })?;

        // Check response status
        if let Some(status) = response.status().as_u16().into() {
            if status == 401u16 {
                return Err("auth_failed".to_string());
            }
        }

        // Store the connection for read_loop
        let mut conn = self.connected.lock().await;
        *conn = true;
        drop(conn);

        // Spawn read loop with the stream
        let (mut _write, mut read) = ws_stream.split();
        let event_tx = self.event_tx.clone();
        let shutdown = self.shutdown.clone();
        let reconnect = self.reconnect_trigger.clone();

        loop {
            tokio::select! {
                _ = shutdown.notified() => return Ok(()),
                _ = reconnect.notified() => return Ok(()),
                msg = read.next() => {
                    match msg {
                        Some(Ok(Message::Text(text))) => {
                            self.handle_message(&text, &event_tx);
                        }
                        Some(Ok(Message::Close(_))) => return Ok(()),
                        Some(Err(e)) => {
                            warn!("[ws] read error: {}", e);
                            return Ok(());
                        }
                        None => return Ok(()),
                        _ => {}
                    }
                }
            }
        }
    }

    async fn read_loop(&self) {
        // read_loop is handled inside dial() now
    }

    fn handle_message(&self, text: &str, event_tx: &mpsc::UnboundedSender<WsClientEvent>) {
        let event: WsEvent = match serde_json::from_str(text) {
            Ok(e) => e,
            Err(e) => {
                warn!("[ws] failed to parse event: {}", e);
                return;
            }
        };

        match event.r#type.as_str() {
            "push" => {
                if let Ok(push) = serde_json::from_value::<PushEvent>(event.payload) {
                    let _ = event_tx.send(WsClientEvent::Push(push));
                }
            }
            "ack_sync" => {
                if let Some(msg_id) = event.payload.get("message_id").and_then(|v| v.as_str()) {
                    let _ = event_tx.send(WsClientEvent::AckSync {
                        message_id: msg_id.to_string(),
                    });
                }
            }
            _ => {}
        }
    }
}
