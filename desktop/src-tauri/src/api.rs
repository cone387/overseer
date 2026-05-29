use reqwest::Client;
use serde::{Deserialize, Serialize};

#[derive(Debug, Serialize)]
struct RegisterRequest {
    name: String,
    token: String,
}

#[derive(Debug, Deserialize)]
struct ApiResponse<T> {
    pub code: u32,
    pub message: String,
    pub data: Option<T>,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct DeviceInfo {
    pub id: String,
    pub name: String,
    pub device_key: String,
    #[serde(rename = "type")]
    pub device_type: String,
}

/// Register a desktop device with the Overseer backend.
pub async fn register_device(
    server_url: &str,
    name: &str,
    token: &str,
) -> Result<DeviceInfo, String> {
    let client = Client::new();
    let url = format!("{}/api/devices/desktop-register", server_url.trim_end_matches('/'));

    let resp = client
        .post(&url)
        .json(&RegisterRequest {
            name: name.to_string(),
            token: token.to_string(),
        })
        .timeout(std::time::Duration::from_secs(10))
        .send()
        .await
        .map_err(|e| format!("network error: {}", e))?;

    let status = resp.status();
    let body = resp
        .text()
        .await
        .map_err(|e| format!("read response: {}", e))?;

    if !status.is_success() {
        return Err(format!("registration failed ({}): {}", status, body));
    }

    let api_resp: ApiResponse<DeviceInfo> =
        serde_json::from_str(&body).map_err(|e| format!("parse response: {}", e))?;

    if api_resp.code != 200 {
        return Err(format!("registration error: {}", api_resp.message));
    }

    api_resp.data.ok_or_else(|| "no device data in response".to_string())
}

/// Acknowledge a message.
pub async fn ack_message(server_url: &str, message_id: &str) -> Result<(), String> {
    let client = Client::new();
    let url = format!(
        "{}/api/messages/{}/ack",
        server_url.trim_end_matches('/'),
        message_id
    );

    let mut last_err = String::new();
    for attempt in 0..3 {
        if attempt > 0 {
            tokio::time::sleep(std::time::Duration::from_millis(
                1000 * 2u64.pow(attempt as u32),
            ))
            .await;
        }

        match client
            .post(&url)
            .timeout(std::time::Duration::from_secs(10))
            .send()
            .await
        {
            Ok(resp) if resp.status().is_success() => return Ok(()),
            Ok(resp) => {
                last_err = format!("ack failed ({})", resp.status());
            }
            Err(e) => {
                last_err = format!("ack network error: {}", e);
            }
        }
    }

    Err(last_err)
}

/// Snooze a message.
pub async fn snooze_message(
    server_url: &str,
    message_id: &str,
    duration: &str,
) -> Result<(), String> {
    let client = Client::new();
    let url = format!(
        "{}/api/messages/{}/snooze",
        server_url.trim_end_matches('/'),
        message_id
    );

    let mut last_err = String::new();
    for attempt in 0..3 {
        if attempt > 0 {
            tokio::time::sleep(std::time::Duration::from_millis(
                1000 * 2u64.pow(attempt as u32),
            ))
            .await;
        }

        match client
            .post(&url)
            .json(&serde_json::json!({ "duration": duration }))
            .timeout(std::time::Duration::from_secs(10))
            .send()
            .await
        {
            Ok(resp) if resp.status().is_success() => return Ok(()),
            Ok(resp) => {
                last_err = format!("snooze failed ({})", resp.status());
            }
            Err(e) => {
                last_err = format!("snooze network error: {}", e);
            }
        }
    }

    Err(last_err)
}
