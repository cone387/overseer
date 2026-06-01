use std::collections::HashMap;
use std::sync::LazyLock;

/// Detected language code.
pub static LANG: LazyLock<String> = LazyLock::new(detect_language);

/// Active translation strings.
static STRINGS: LazyLock<&'static HashMap<&'static str, &'static str>> = LazyLock::new(|| {
    if LANG.as_str() == "zh" {
        &*ZH_STRINGS
    } else {
        &*EN_STRINGS
    }
});

/// Get localized string by key.
pub fn t(key: &'static str) -> &'static str {
    STRINGS
        .get(key)
        .or_else(|| EN_STRINGS.get(key))
        .copied()
        .unwrap_or(key)
}

fn detect_language() -> String {
    // Try sys-locale first
    if let Some(locale) = sys_locale::get_locale() {
        if locale.to_lowercase().starts_with("zh") {
            return "zh".to_string();
        }
        return "en".to_string();
    }

    // Fallback: check environment variables
    for var in &["LANG", "LANGUAGE", "LC_ALL", "LC_MESSAGES"] {
        if let Ok(val) = std::env::var(var) {
            if val.to_lowercase().starts_with("zh") {
                return "zh".to_string();
            }
            return "en".to_string();
        }
    }

    "zh".to_string()
}

static ZH_STRINGS: LazyLock<HashMap<&'static str, &'static str>> = LazyLock::new(|| {
    let mut m = HashMap::new();
    // Tray menu
    m.insert("tray.connected", "● 已连接");
    m.insert("tray.disconnected", "○ 已断开");
    m.insert("tray.mute", "静音");
    m.insert("tray.autostart", "开机自启");
    m.insert("tray.settings", "设置");
    m.insert("tray.open_webui", "打开控制台");
    m.insert("tray.reconnect", "重新连接");
    m.insert("tray.quit", "退出");
    m.insert("tray.recent", "最近通知");
    m.insert("tray.view_all", "查看全部");
    m.insert("tray.no_history", "暂无通知");
    // Setup
    m.insert("setup.title", "Overseer Desktop - 首次配置");
    m.insert("setup.server_url", "服务器地址");
    m.insert("setup.token", "注册令牌");
    m.insert("setup.device_name", "设备名称（可选）");
    m.insert("setup.connect", "连接");
    m.insert("setup.cancel", "取消");
    m.insert("setup.validation", "请填写服务器地址和注册令牌");
    // Notifications
    m.insert("notify.update_title", "发现新版本");
    m.insert("notify.update_message", "Overseer Desktop {} 已发布，点击下载更新");
    // Status
    m.insert("status.muted", "通知已静音");
    m.insert("status.unmuted", "通知已取消静音");
    m
});

static EN_STRINGS: LazyLock<HashMap<&'static str, &'static str>> = LazyLock::new(|| {
    let mut m = HashMap::new();
    // Tray menu
    m.insert("tray.connected", "● Connected");
    m.insert("tray.disconnected", "○ Disconnected");
    m.insert("tray.mute", "Mute");
    m.insert("tray.autostart", "Start on Boot");
    m.insert("tray.settings", "Settings");
    m.insert("tray.open_webui", "Open Web UI");
    m.insert("tray.reconnect", "Reconnect");
    m.insert("tray.quit", "Quit");
    m.insert("tray.recent", "Recent Notifications");
    m.insert("tray.view_all", "View All");
    m.insert("tray.no_history", "No notifications");
    // Setup
    m.insert("setup.title", "Overseer Desktop - Setup");
    m.insert("setup.server_url", "Server URL");
    m.insert("setup.token", "Registration Token");
    m.insert("setup.device_name", "Device Name (optional)");
    m.insert("setup.connect", "Connect");
    m.insert("setup.cancel", "Cancel");
    m.insert("setup.validation", "Please fill in server URL and registration token");
    // Notifications
    m.insert("notify.update_title", "Update Available");
    m.insert("notify.update_message", "Overseer Desktop {} is available. Click to download.");
    // Status
    m.insert("status.muted", "Notifications muted");
    m.insert("status.unmuted", "Notifications unmuted");
    m
});
