use std::fs;
use std::path::PathBuf;

/// Check if autostart is enabled.
pub fn is_enabled() -> bool {
    plist_path().map(|p| p.exists()).unwrap_or(false)
}

/// Enable autostart (macOS: LaunchAgent plist).
pub fn enable() -> Result<(), String> {
    let path = plist_path().ok_or("cannot determine plist path")?;
    let dir = path.parent().ok_or("cannot determine LaunchAgents dir")?;

    fs::create_dir_all(dir).map_err(|e| format!("create dir: {}", e))?;

    let exe_path = std::env::current_exe()
        .map_err(|e| format!("get exe path: {}", e))?
        .to_string_lossy()
        .to_string();

    let plist = format!(
        r#"<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.overseer.desktop</string>
    <key>ProgramArguments</key>
    <array>
        <string>{}</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <false/>
</dict>
</plist>
"#,
        exe_path
    );

    fs::write(&path, plist).map_err(|e| format!("write plist: {}", e))?;
    Ok(())
}

/// Disable autostart.
pub fn disable() -> Result<(), String> {
    let path = plist_path().ok_or("cannot determine plist path")?;
    if path.exists() {
        fs::remove_file(&path).map_err(|e| format!("remove plist: {}", e))?;
    }
    Ok(())
}

fn plist_path() -> Option<PathBuf> {
    dirs::home_dir().map(|h| {
        h.join("Library")
            .join("LaunchAgents")
            .join("com.overseer.desktop.plist")
    })
}
