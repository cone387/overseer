use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use std::fs;
use std::path::PathBuf;

/// Desktop client persistent configuration.
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct Config {
    #[serde(default)]
    pub server_url: String,
    #[serde(default)]
    pub api_key: String,
    #[serde(default)]
    pub device_id: String,
    #[serde(default)]
    pub device_name: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub last_update_check: Option<DateTime<Utc>>,
}

impl Config {
    /// Returns the platform-specific config directory.
    fn config_dir() -> Option<PathBuf> {
        dirs::config_dir().map(|d| d.join("overseer-desktop"))
    }

    /// Returns the full path to the config file.
    fn config_path() -> Option<PathBuf> {
        Self::config_dir().map(|d| d.join("config.json"))
    }

    /// Load config from disk. Returns default if file doesn't exist or is corrupted.
    pub fn load() -> Self {
        let path = match Self::config_path() {
            Some(p) => p,
            None => return Self::default(),
        };

        match fs::read_to_string(&path) {
            Ok(data) => serde_json::from_str(&data).unwrap_or_default(),
            Err(_) => Self::default(),
        }
    }

    /// Save config to disk.
    pub fn save(&self) -> Result<(), String> {
        let path = Self::config_path().ok_or("cannot determine config path")?;
        let dir = path.parent().ok_or("cannot determine config dir")?;

        fs::create_dir_all(dir).map_err(|e| format!("create config dir: {}", e))?;

        let data =
            serde_json::to_string_pretty(self).map_err(|e| format!("serialize config: {}", e))?;

        fs::write(&path, data).map_err(|e| format!("write config: {}", e))?;
        Ok(())
    }

    /// Check if the config has valid credentials.
    pub fn is_configured(&self) -> bool {
        !self.server_url.is_empty() && !self.api_key.is_empty()
    }
}
