use futures_util::{SinkExt, StreamExt};
use log::{error, info, warn};
use serde::{Deserialize, Serialize};
use std::sync::Arc;
use std::time::Duration;
use tauri::{AppHandle, Emitter};
use tokio::sync::{watch, Mutex};
use tokio_tungstenite::{connect_async, tungstenite::Message};
use url::Url;

#[derive(Debug, Clone, Serialize, Deserialize)]
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

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AckSyncEvent {
    pub message_id: String,
}

#[derive(Debug, Deserialize)]
struct WsEvent {
    #[serde(rename = "type")]
    event_type: String,
    payload: serde_json::Value,
}

#[derive(Debug, Clone, PartialEq, Serialize)]
pub enum ConnectionState {
    Connected,
    Disconnected,
    Connecting,
}

pub struct WsClient {
    shutdown_tx: watch::Sender<bool>,
    reconnect_tx: tokio::sync::mpsc::Sender<()>,
    state: Arc<Mutex<ConnectionState>>,
}

impl WsClient {
    pub fn new(
        app_handle: AppHandle,
        server_url: String,
        api_key: String,
    ) -> Self {
        let (shutdown_tx, shutdown_rx) = watch::channel(false);
        let (reconnect_tx, reconnect_rx) = tokio::sync::mpsc::channel(1);
        let state = Arc::new(Mutex::new(ConnectionState::Disconnected));

        let state_clone = state.clone();
        let app_clone = app_handle.clone();

        tokio::spawn(async move {
            Self::connection_loop(
                app_clone,
                server_url,
                api_key,
                shutdown_rx,
                reconnect_rx,
                state_clone,
            )
            .await;
        });

        Self {
            shutdown_tx,
            reconnect_tx,
            state,
        }
    }

    pub async fn is_connected(&self) -> bool {
        *self.state.lock().await == ConnectionState::Connected
    }

    pub fn reconnect(&self) {
        let _ = self.reconnect_tx.try_send(());
    }

    pub fn shutdown(&self) {
        let _ = self.shutdown_tx.send(true);
    }

    async fn connection_loop(
        app: AppHandle,
        server_url: String,
        api_key: String,
        mut shutdown_rx: watch::Receiver<bool>,
        mut reconnect_rx: tokio::sync::mpsc::Receiver<()>,
        state: Arc<Mutex<ConnectionState>>,
    ) {
        let mut backoff = Duration::from_secs(1);
        let max_backoff = Duration::from_secs(60);

        loop {
            // Check shutdown
            if *shutdown_rx.borrow() {
                return;
            }

            // Build WebSocket URL
            let ws_url = match Self::build_ws_url(&server_url, &api_key) {
                Ok(url) => url,
                Err(e) => {
                    error!("[ws] invalid URL: {}", e);
                    return;
                }
            };

            // Update state
            {
                let mut s = state.lock().await;
                *s = ConnectionState::Connecting;
            }
            let _ = app.emit("ws-state", "connecting");

            info!("[ws] connecting to {}...", server_url);

            match connect_async(&ws_url).await {
                Ok((ws_stream, response)) => {
                    // Check for 401
                    if response.status().as_u16() == 401 {
                        error!("[ws] unauthorized (401)");
                        let _ = app.emit("ws-auth-fail", ());
                        return;
                    }

                    info!("[ws] connected");
                    backoff = Duration::from_secs(1); // Reset backoff

                    {
                        let mut s = state.lock().await;
                        *s = ConnectionState::Connected;
                    }
                    let _ = app.emit("ws-state", "connected");

                    let (mut write, mut read) = ws_stream.split();

                    // Read loop
                    loop {
                        tokio::select! {
                            _ = shutdown_rx.changed() => {
                                if *shutdown_rx.borrow() {
                                    let _ = write.close().await;
                                    return;
                                }
                            }
                            _ = reconnect_rx.recv() => {
                                info!("[ws] reconnect requested");
                                let _ = write.close().await;
                                break;
                            }
                            msg = read.next() => {
                                match msg {
                                    Some(Ok(Message::Text(text))) => {
                                        Self::handle_message(&app, &text);
                                    }
                                    Some(Ok(Message::Ping(data))) => {
                                        let _ = write.send(Message::Pong(data)).await;
                                    }
                                    Some(Ok(Message::Close(_))) => {
                                        info!("[ws] server closed connection");
                                        break;
                                    }
                                    Some(Err(e)) => {
                                        warn!("[ws] read error: {}", e);
                                        break;
                                    }
                                    None => {
                                        info!("[ws] stream ended");
                                        break;
                                    }
                                    _ => {}
                                }
                            }
                        }
                    }
                }
                Err(e) => {
                    // Check if it's a 401 error
                    let err_str = e.to_string();
                    if err_str.contains("401") {
                        error!("[ws] unauthorized (401)");
                        let _ = app.emit("ws-auth-fail", ());
                        // Don't retry on auth failure
                        {
                            let mut s = state.lock().await;
                            *s = ConnectionState::Disconnected;
                        }
                        let _ = app.emit("ws-state", "disconnected");
                        return;
                    }
                    warn!("[ws] connection failed: {} (retry in {:?})", e, backoff);
                }
            }

            // Disconnected
            {
                let mut s = state.lock().await;
                *s = ConnectionState::Disconnected;
            }
            let _ = app.emit("ws-state", "disconnected");

            // Wait with backoff
            tokio::select! {
                _ = tokio::time::sleep(backoff) => {}
                _ = shutdown_rx.changed() => {
                    if *shutdown_rx.borrow() {
                        return;
                    }
                }
                _ = reconnect_rx.recv() => {
                    // Immediate reconnect requested
                    backoff = Duration::from_secs(1);
                    continue;
                }
            }

            // Exponential backoff: 1s -> 2s -> 4s -> ... -> 60s
            backoff = std::cmp::min(backoff * 2, max_backoff);
        }
    }

    fn build_ws_url(server_url: &str, api_key: &str) -> Result<String, String> {
        let mut url = Url::parse(server_url).map_err(|e| e.to_string())?;

        // Convert http(s) to ws(s)
        match url.scheme() {
            "https" => url.set_scheme("wss").map_err(|_| "set scheme failed")?,
            _ => url.set_scheme("ws").map_err(|_| "set scheme failed")?,
        }

        url.set_path("/ws");
        url.query_pairs_mut().append_pair("api_key", api_key);

        Ok(url.to_string())
    }

    fn handle_message(app: &AppHandle, text: &str) {
        let event: WsEvent = match serde_json::from_str(text) {
            Ok(e) => e,
            Err(e) => {
                warn!("[ws] failed to parse event: {}", e);
                return;
            }
        };

        match event.event_type.as_str() {
            "push" => {
                let push: PushEvent = match serde_json::from_value(event.payload) {
                    Ok(p) => p,
                    Err(e) => {
                        warn!("[ws] failed to parse push payload: {}", e);
                        return;
                    }
                };
                info!(
                    "[ws] push: [{}] {} - {}",
                    push.channel, push.title, push.body
                );
                let _ = app.emit("ws-push", &push);
            }
            "ack_sync" => {
                let ack: AckSyncEvent = match serde_json::from_value(event.payload) {
                    Ok(a) => a,
                    Err(e) => {
                        warn!("[ws] failed to parse ack_sync payload: {}", e);
                        return;
                    }
                };
                info!("[ws] ack_sync: {}", ack.message_id);
                let _ = app.emit("ws-ack-sync", &ack);
            }
            _ => {
                info!("[ws] unknown event type: {}", event.event_type);
            }
        }
    }
}
