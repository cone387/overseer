use chrono::{DateTime, Utc};
use log::{info, warn};
use reqwest::Client;
use serde::Deserialize;

const REPO_OWNER: &str = "cone387";
const REPO_NAME: &str = "overseer";
const CHECK_INTERVAL_HOURS: i64 = 24;

#[derive(Debug, Deserialize)]
struct GithubRelease {
    tag_name: String,
    html_url: String,
    #[allow(dead_code)]
    name: Option<String>,
}

#[derive(Debug, Clone)]
pub struct UpdateResult {
    pub version: String,
    pub download_url: String,
}

/// Check GitHub for a newer version.
/// Returns None if no update is available or check should be skipped.
pub async fn check_update(
    current_version: &str,
    last_check: Option<&str>,
) -> Option<UpdateResult> {
    // Skip if version is "dev" or empty
    if current_version.is_empty() || current_version == "dev" {
        return None;
    }

    // Skip if checked recently
    if let Some(last) = last_check {
        if let Ok(last_time) = DateTime::parse_from_rfc3339(last) {
            let elapsed = Utc::now().signed_duration_since(last_time);
            if elapsed.num_hours() < CHECK_INTERVAL_HOURS {
                return None;
            }
        }
    }

    let url = format!(
        "https://api.github.com/repos/{}/{}/releases/latest",
        REPO_OWNER, REPO_NAME
    );

    let client = Client::new();
    let resp = match client
        .get(&url)
        .header("User-Agent", "overseer-desktop")
        .timeout(std::time::Duration::from_secs(10))
        .send()
        .await
    {
        Ok(r) => r,
        Err(e) => {
            warn!("[updater] failed to check for updates: {}", e);
            return None;
        }
    };

    if !resp.status().is_success() {
        return None;
    }

    let release: GithubRelease = match resp.json().await {
        Ok(r) => r,
        Err(e) => {
            warn!("[updater] failed to parse release: {}", e);
            return None;
        }
    };

    // Compare versions (strip "desktop-v" prefix)
    let latest = release.tag_name.trim_start_matches("desktop-v");
    let current = current_version.trim_start_matches("desktop-v");

    if is_newer(latest, current) {
        info!("[updater] new version available: {}", release.tag_name);
        Some(UpdateResult {
            version: release.tag_name,
            download_url: release.html_url,
        })
    } else {
        None
    }
}

fn is_newer(a: &str, b: &str) -> bool {
    // Try semver comparison first
    if let (Ok(va), Ok(vb)) = (semver::Version::parse(a), semver::Version::parse(b)) {
        return va > vb;
    }

    // Fallback to simple string comparison
    let a_parts: Vec<&str> = a.split('.').collect();
    let b_parts: Vec<&str> = b.split('.').collect();

    for i in 0..a_parts.len().max(b_parts.len()) {
        let a_part = a_parts.get(i).unwrap_or(&"0");
        let b_part = b_parts.get(i).unwrap_or(&"0");

        let a_num = a_part.parse::<u32>().unwrap_or(0);
        let b_num = b_part.parse::<u32>().unwrap_or(0);

        if a_num > b_num {
            return true;
        }
        if a_num < b_num {
            return false;
        }
    }
    false
}
