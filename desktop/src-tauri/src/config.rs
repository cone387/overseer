use serde::{Deserialize, Serialize};
use std::fs;
use std::path::PathBuf;

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct AppConfig {
    pub server_url: String,
    pub api_key: String,
    pub device_id: String,
    pub device_name: String,
    pub last_update_check: Option<String>,
    pub language: Option<String>,
    pub muted: Option<bool>,
}

impl AppConfig {
    pub fn config_dir() -> PathBuf {
        let base = dirs::data_local_dir().unwrap_or_else(|| PathBuf::from("."));
        base.join("overseer-desktop")
    }

    pub fn config_path() -> PathBuf {
        Self::config_dir().join("config.json")
    }

    pub fn load() -> Self {
        let path = Self::config_path();
        if let Ok(data) = fs::read_to_string(&path) {
            serde_json::from_str(&data).unwrap_or_default()
        } else {
            Self::default()
        }
    }

    pub fn save(&self) -> Result<(), String> {
        let dir = Self::config_dir();
        fs::create_dir_all(&dir).map_err(|e| format!("create config dir: {}", e))?;
        let path = Self::config_path();
        let data =
            serde_json::to_string_pretty(self).map_err(|e| format!("serialize config: {}", e))?;
        fs::write(&path, data).map_err(|e| format!("write config: {}", e))?;
        Ok(())
    }

    pub fn is_configured(&self) -> bool {
        !self.server_url.is_empty() && !self.api_key.is_empty()
    }
}
