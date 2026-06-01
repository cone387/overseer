use chrono::{DateTime, Duration, Utc};
use log::{info, warn};
use reqwest::Client;
use serde::Deserialize;

const REPO_OWNER: &str = "cone387";
const REPO_NAME: &str = "overseer";

/// Update check result.
#[derive(Debug, Clone)]
pub struct UpdateResult {
    pub available: bool,
    pub version: String,
    pub download_url: String,
}

#[derive(Deserialize)]
struct GithubRelease {
    tag_name: String,
    html_url: String,
}

/// Check GitHub for a newer version.
/// Returns None if no update available or check should be skipped.
pub async fn check(current_version: &str, last_check: Option<DateTime<Utc>>) -> Option<UpdateResult> {
    // Skip if checked within 24 hours
    if let Some(last) = last_check {
        if Utc::now() - last < Duration::hours(24) {
            return None;
        }
    }

    // Skip dev builds
    if current_version.is_empty() || current_version == "dev" {
        return None;
    }

    let url = format!(
        "https://api.github.com/repos/{}/{}/releases/latest",
        REPO_OWNER, REPO_NAME
    );

    let client = Client::builder()
        .timeout(std::time::Duration::from_secs(10))
        .user_agent("overseer-desktop")
        .build()
        .ok()?;

    let resp = match client.get(&url).send().await {
        Ok(r) => r,
        Err(e) => {
            warn!("[updater] failed to check for updates: {}", e);
            return None;
        }
    };

    if resp.status() != 200 {
        return None;
    }

    let release: GithubRelease = match resp.json().await {
        Ok(r) => r,
        Err(_) => return None,
    };

    let latest = release.tag_name.trim_start_matches("desktop-v");
    let current = current_version.trim_start_matches("desktop-v");

    if is_newer(latest, current) {
        info!("[updater] new version available: {}", release.tag_name);
        Some(UpdateResult {
            available: true,
            version: release.tag_name,
            download_url: release.html_url,
        })
    } else {
        None
    }
}

/// Simple version comparison: a > b
fn is_newer(a: &str, b: &str) -> bool {
    let a_parts: Vec<&str> = a.split('.').collect();
    let b_parts: Vec<&str> = b.split('.').collect();

    for i in 0..a_parts.len().max(b_parts.len()) {
        let av = a_parts.get(i).and_then(|s| s.parse::<u32>().ok()).unwrap_or(0);
        let bv = b_parts.get(i).and_then(|s| s.parse::<u32>().ok()).unwrap_or(0);
        if av > bv {
            return true;
        }
        if av < bv {
            return false;
        }
    }
    false
}
