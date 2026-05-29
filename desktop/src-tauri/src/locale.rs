use std::collections::HashMap;
use std::sync::LazyLock;

static ZH_STRINGS: LazyLock<HashMap<&'static str, &'static str>> = LazyLock::new(|| {
    let mut m = HashMap::new();
    // Tray menu
    m.insert("tray.connected", "● 已连接");
    m.insert("tray.disconnected", "○ 已断开");
    m.insert("tray.mute", "静音");
    m.insert("tray.unmute", "取消静音");
    m.insert("tray.autostart", "开机自启");
    m.insert("tray.settings", "设置");
    m.insert("tray.open_webui", "打开控制台");
    m.insert("tray.reconnect", "重新连接");
    m.insert("tray.quit", "退出");
    m.insert("tray.recent", "最近通知");
    m.insert("tray.view_all", "查看全部");
    m.insert("tray.no_history", "暂无通知");
    m.insert("tray.messages", "消息列表");
    // Notifications
    m.insert("notify.update_title", "发现新版本");
    m.insert("notify.update_message", "Overseer Desktop {} 已发布，点击下载更新");
    m.insert("notify.ack", "确认");
    m.insert("notify.snooze", "稍后");
    m.insert("notify.open", "打开");
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
    m.insert("tray.unmute", "Unmute");
    m.insert("tray.autostart", "Start on Boot");
    m.insert("tray.settings", "Settings");
    m.insert("tray.open_webui", "Open Web UI");
    m.insert("tray.reconnect", "Reconnect");
    m.insert("tray.quit", "Quit");
    m.insert("tray.recent", "Recent Notifications");
    m.insert("tray.view_all", "View All");
    m.insert("tray.no_history", "No notifications");
    m.insert("tray.messages", "Messages");
    // Notifications
    m.insert("notify.update_title", "Update Available");
    m.insert("notify.update_message", "Overseer Desktop {} is available. Click to download.");
    m.insert("notify.ack", "Acknowledge");
    m.insert("notify.snooze", "Snooze");
    m.insert("notify.open", "Open");
    // Status
    m.insert("status.muted", "Notifications muted");
    m.insert("status.unmuted", "Notifications unmuted");
    m
});

static CURRENT_LANG: LazyLock<String> = LazyLock::new(detect_language);

fn detect_language() -> String {
    // Check Windows locale
    #[cfg(target_os = "windows")]
    {
        if let Ok(output) = std::process::Command::new("powershell")
            .args(["-NoProfile", "-Command", "[System.Globalization.CultureInfo]::CurrentUICulture.TwoLetterISOLanguageName"])
            .output()
        {
            let lang = String::from_utf8_lossy(&output.stdout).trim().to_string();
            if lang.starts_with("zh") {
                return "zh".to_string();
            }
        }
    }

    // Check environment variables
    for var in &["LANG", "LANGUAGE", "LC_ALL", "LC_MESSAGES"] {
        if let Ok(val) = std::env::var(var) {
            if val.to_lowercase().starts_with("zh") {
                return "zh".to_string();
            }
            return "en".to_string();
        }
    }

    "zh".to_string() // default to Chinese
}

pub fn current_lang() -> &'static str {
    &CURRENT_LANG
}

pub fn t(key: &str) -> &'static str {
    let lang = current_lang();
    let strings = if lang == "zh" {
        &*ZH_STRINGS
    } else {
        &*EN_STRINGS
    };

    if let Some(s) = strings.get(key) {
        s
    } else if let Some(s) = EN_STRINGS.get(key) {
        s
    } else {
        // Leak the key to get a 'static reference (only happens for unknown keys)
        Box::leak(key.to_string().into_boxed_str())
    }
}
