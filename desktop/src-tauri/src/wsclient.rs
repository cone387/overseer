use futures_util::{SinkExt, StreamExt};
use log::{error, info, warn};
use serde::Deserialize;
use std::time::Duration;
use tokio::sync::mpsc;
use tokio_tungstenite::{connect_async, tungstenite::Message};
use url::Url;

#[derive(Debug, Clone)]
pub struct WsPushEvent {
    pub id: String,
    pub source: String,
    pub channel: String,
    pub title: String,
    pub body: String,
    pub url: String,
    pub icon: String,
    pub level: String,
    pub status: String,
    pub expires_at: String,
}

#[derive(Debug, Clone)]
pub enum WsClientEvent {
    Push(WsPushEvent),
    AckSync { message_id: String },
    Connected,
    Disconnected,
    AuthFailed,
}

#[derive(Debug, Deserialize)]
struct WsMessage {
    #[serde(rename = "type")]
    event_type: String,
    payload: serde_json::Value,
}

#[derive(Debug, Deserialize)]
struct PushPayload {
    #[serde(default)]
    id: String,
    #[serde(default)]
    source: String,
    #[serde(default)]
    channel: String,
    #[serde(default)]
    title: String,
    #[serde(default)]
    body: String,
    #[serde(default)]
    url: String,
    #[serde(default)]
    icon: String,
    #[serde(default)]
    level: String,
    #[serde(default)]
    status: String,
    #[serde(default)]
    expires_at: String,
}

#[derive(Debug, Deserialize)]
struct AckSyncPayload {
    message_id: String,
}

pub struct WsClient {
    server_url: String,
    api_key: String,
    event_tx: mpsc::UnboundedSender<WsClientEvent>,
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
        }
    }

    pub async fn run(&self) {
        let mut backoff = Duration::from_secs(1);
        let max_backoff = Duration::from_secs(60);

        loop {
            let ws_url = match self.build_ws_url() {
                Ok(url) => url,
                Err(e) => {
                    error!("[ws] invalid URL: {}", e);
                    return;
                }
            };

            info!("[ws] connecting to {}...", self.server_url);

            match connect_async(&ws_url).await {
                Ok((ws_stream, response)) => {
                    if response.status().as_u16() == 401 {
                        error!("[ws] unauthorized (401)");
                        let _ = self.event_tx.send(WsClientEvent::AuthFailed);
                        return;
                    }

                    info!("[ws] connected");
                    backoff = Duration::from_secs(1);
                    let _ = self.event_tx.send(WsClientEvent::Connected);

                    let (mut write, mut read) = ws_stream.split();

                    loop {
                        match read.next().await {
                            Some(Ok(Message::Text(text))) => {
                                self.handle_message(&text);
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
                Err(e) => {
                    let err_str = e.to_string();
                    if err_str.contains("401") {
                        error!("[ws] unauthorized (401)");
                        let _ = self.event_tx.send(WsClientEvent::AuthFailed);
                        return;
                    }
                    warn!("[ws] connection failed: {} (retry in {:?})", e, backoff);
                }
            }

            let _ = self.event_tx.send(WsClientEvent::Disconnected);

            tokio::time::sleep(backoff).await;
            backoff = std::cmp::min(backoff * 2, max_backoff);
        }
    }

    fn build_ws_url(&self) -> Result<String, String> {
        let mut url = Url::parse(&self.server_url).map_err(|e| e.to_string())?;

        match url.scheme() {
            "https" => url.set_scheme("wss").map_err(|_| "set scheme failed")?,
            _ => url.set_scheme("ws").map_err(|_| "set scheme failed")?,
        }

        url.set_path("/ws");
        url.query_pairs_mut().append_pair("api_key", &self.api_key);

        Ok(url.to_string())
    }

    fn handle_message(&self, text: &str) {
        let event: WsMessage = match serde_json::from_str(text) {
            Ok(e) => e,
            Err(e) => {
                warn!("[ws] failed to parse event: {}", e);
                return;
            }
        };

        match event.event_type.as_str() {
            "push" => {
                let payload: PushPayload = match serde_json::from_value(event.payload) {
                    Ok(p) => p,
                    Err(e) => {
                        warn!("[ws] failed to parse push payload: {}", e);
                        return;
                    }
                };
                info!("[ws] push: [{}] {} - {}", payload.channel, payload.title, payload.body);
                let _ = self.event_tx.send(WsClientEvent::Push(WsPushEvent {
                    id: payload.id,
                    source: payload.source,
                    channel: payload.channel,
                    title: payload.title,
                    body: payload.body,
                    url: payload.url,
                    icon: payload.icon,
                    level: payload.level,
                    status: payload.status,
                    expires_at: payload.expires_at,
                }));
            }
            "ack_sync" => {
                let payload: AckSyncPayload = match serde_json::from_value(event.payload) {
                    Ok(a) => a,
                    Err(e) => {
                        warn!("[ws] failed to parse ack_sync payload: {}", e);
                        return;
                    }
                };
                info!("[ws] ack_sync: {}", payload.message_id);
                let _ = self.event_tx.send(WsClientEvent::AckSync {
                    message_id: payload.message_id,
                });
            }
            _ => {
                info!("[ws] unknown event type: {}", event.event_type);
            }
        }
    }
}
