use reqwest::Client;
use serde::{Deserialize, Serialize};
use std::time::Duration;

/// Device registration response from the server.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Device {
    pub id: String,
    pub name: String,
    pub device_key: String,
    #[serde(default)]
    pub r#type: String,
    #[serde(default)]
    pub is_default: bool,
}

/// Standard API response envelope.
#[derive(Debug, Deserialize)]
struct ApiResponse {
    code: i32,
    message: String,
    data: Option<serde_json::Value>,
}

/// Registration request body.
#[derive(Serialize)]
struct RegisterRequest {
    name: String,
    token: String,
}

/// HTTP client with 10s timeout.
fn http_client() -> Client {
    Client::builder()
        .timeout(Duration::from_secs(10))
        .build()
        .unwrap_or_default()
}

/// Initiate desktop link flow. Returns a link code.
/// POST /api/devices/desktop-link
pub async fn desktop_link(base_url: &str, name: &str) -> Result<String, String> {
    let url = format!("{}/api/devices/desktop-link", base_url);
    let body = serde_json::json!({"name": name});

    let resp = http_client()
        .post(&url)
        .json(&body)
        .send()
        .await
        .map_err(|e| format!("request failed: {}", e))?;

    let resp_body = resp.text().await.map_err(|e| format!("read response: {}", e))?;
    let api_resp: ApiResponse =
        serde_json::from_str(&resp_body).map_err(|e| format!("parse: {}", e))?;

    if api_resp.code != 200 {
        return Err(format!("{} (code {})", api_resp.message, api_resp.code));
    }

    let data = api_resp.data.ok_or("no data")?;
    data.get("link_code")
        .and_then(|v| v.as_str())
        .map(|s| s.to_string())
        .ok_or_else(|| "no link_code in response".to_string())
}

/// Poll for desktop link confirmation.
/// GET /api/devices/desktop-poll?code=xxx
/// Returns Ok(Some(device)) when confirmed, Ok(None) when still pending.
pub async fn desktop_poll(base_url: &str, code: &str) -> Result<Option<Device>, String> {
    let url = format!("{}/api/devices/desktop-poll?code={}", base_url, code);

    let resp = http_client()
        .get(&url)
        .send()
        .await
        .map_err(|e| format!("request failed: {}", e))?;

    let status = resp.status().as_u16();
    let resp_body = resp.text().await.map_err(|e| format!("read response: {}", e))?;

    if status == 202 {
        // Still pending
        return Ok(None);
    }

    if status == 404 || status == 410 {
        return Err("link code expired or not found".to_string());
    }

    let api_resp: ApiResponse =
        serde_json::from_str(&resp_body).map_err(|e| format!("parse: {}", e))?;

    if api_resp.code != 200 {
        return Err(format!("{}", api_resp.message));
    }

    let data = api_resp.data.ok_or("no data")?;
    let device: Device = serde_json::from_value(data).map_err(|e| format!("parse device: {}", e))?;
    Ok(Some(device))
}

/// Register this desktop client with the server (legacy token-based).
/// POST /api/devices/desktop-register
pub async fn register(base_url: &str, name: &str, token: &str) -> Result<Device, String> {
    let url = format!("{}/api/devices/desktop-register", base_url);
    let body = RegisterRequest {
        name: name.to_string(),
        token: token.to_string(),
    };

    let resp = http_client()
        .post(&url)
        .json(&body)
        .send()
        .await
        .map_err(|e| format!("request failed: {}", e))?;

    let resp_body = resp
        .text()
        .await
        .map_err(|e| format!("read response: {}", e))?;

    let api_resp: ApiResponse =
        serde_json::from_str(&resp_body).map_err(|e| format!("parse response: {} (body: {})", e, resp_body))?;

    if api_resp.code != 200 {
        return Err(format!(
            "registration failed: {} (code {})",
            api_resp.message, api_resp.code
        ));
    }

    let data = api_resp.data.ok_or("no data in response")?;
    let device: Device =
        serde_json::from_value(data).map_err(|e| format!("parse device data: {}", e))?;

    Ok(device)
}

/// POST with exponential backoff retry (max 3 attempts, 1s→2s→4s).
pub async fn post_with_retry(url: &str, body: Option<&[u8]>, api_key: &str) -> Result<(), String> {
    let client = http_client();
    let mut backoff = Duration::from_secs(1);

    for attempt in 0..3 {
        let mut req = client.post(url)
            .header("X-Device-Key", api_key);
        if let Some(b) = body {
            req = req.header("content-type", "application/json").body(b.to_vec());
        }

        match req.send().await {
            Ok(resp) => {
                let status = resp.status().as_u16();
                if (200..300).contains(&status) {
                    return Ok(());
                }
                if attempt == 2 {
                    return Err(format!("HTTP {}", status));
                }
            }
            Err(e) => {
                if attempt == 2 {
                    return Err(format!("request failed after 3 attempts: {}", e));
                }
            }
        }

        tokio::time::sleep(backoff).await;
        backoff *= 2;
    }

    Err("request failed after 3 attempts".to_string())
}

/// Acknowledge a message.
pub async fn ack_message(base_url: &str, api_key: &str, message_id: &str) -> Result<(), String> {
    let url = format!("{}/api/messages/{}/ack", base_url, message_id);
    post_with_retry(&url, None, api_key).await
}

/// Snooze a message.
pub async fn snooze_message(base_url: &str, api_key: &str, message_id: &str, duration: &str) -> Result<(), String> {
    let url = format!("{}/api/messages/{}/snooze", base_url, message_id);
    let body = serde_json::json!({"duration": duration}).to_string();
    post_with_retry(&url, Some(body.as_bytes()), api_key).await
}
