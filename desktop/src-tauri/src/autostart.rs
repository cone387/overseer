use log::info;
use std::path::PathBuf;

const APP_ID: &str = "com.overseer.desktop";

fn plist_path() -> Option<PathBuf> {
    dirs::home_dir().map(|home| {
        home.join("Library")
            .join("LaunchAgents")
            .join(format!("{}.plist", APP_ID))
    })
}

#[cfg(target_os = "macos")]
fn current_exe_path() -> Option<String> {
    std::env::current_exe()
        .ok()
        .and_then(|p| p.to_str().map(|s| s.to_string()))
}

pub fn is_enabled() -> bool {
    #[cfg(target_os = "macos")]
    {
        plist_path().map(|p| p.exists()).unwrap_or(false)
    }
    #[cfg(target_os = "windows")]
    {
        use winreg::enums::*;
        use winreg::RegKey;
        let hkcu = RegKey::predef(HKEY_CURRENT_USER);
        if let Ok(key) = hkcu.open_subkey("Software\\Microsoft\\Windows\\CurrentVersion\\Run") {
            key.get_value::<String, _>("OverseerDesktop").is_ok()
        } else {
            false
        }
    }
    #[cfg(target_os = "linux")]
    {
        dirs::config_dir()
            .map(|d| d.join("autostart").join("overseer-desktop.desktop").exists())
            .unwrap_or(false)
    }
}

pub fn enable() -> Result<(), String> {
    #[cfg(target_os = "macos")]
    {
        let exe = current_exe_path().ok_or("cannot determine executable path")?;
        let plist = plist_path().ok_or("cannot determine LaunchAgents path")?;

        if let Some(parent) = plist.parent() {
            std::fs::create_dir_all(parent).map_err(|e| e.to_string())?;
        }

        let content = format!(
            r#"<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>{}</string>
    <key>ProgramArguments</key>
    <array>
        <string>{}</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
</dict>
</plist>"#,
            APP_ID, exe
        );

        std::fs::write(&plist, content).map_err(|e| format!("write plist: {}", e))?;
        info!("[autostart] enabled via LaunchAgent");
        Ok(())
    }
    #[cfg(target_os = "windows")]
    {
        use winreg::enums::*;
        use winreg::RegKey;
        let exe = std::env::current_exe().map_err(|e| e.to_string())?;
        let hkcu = RegKey::predef(HKEY_CURRENT_USER);
        let (key, _) = hkcu
            .create_subkey("Software\\Microsoft\\Windows\\CurrentVersion\\Run")
            .map_err(|e| e.to_string())?;
        key.set_value("OverseerDesktop", &exe.to_string_lossy().to_string())
            .map_err(|e| e.to_string())?;
        info!("[autostart] enabled via registry");
        Ok(())
    }
    #[cfg(target_os = "linux")]
    {
        let exe = std::env::current_exe().map_err(|e| e.to_string())?;
        let autostart_dir = dirs::config_dir()
            .ok_or("cannot determine config dir")?
            .join("autostart");
        std::fs::create_dir_all(&autostart_dir).map_err(|e| e.to_string())?;

        let desktop_file = autostart_dir.join("overseer-desktop.desktop");
        let content = format!(
            "[Desktop Entry]\nType=Application\nName=Overseer Desktop\nExec={}\nX-GNOME-Autostart-enabled=true\n",
            exe.to_string_lossy()
        );
        std::fs::write(&desktop_file, content).map_err(|e| e.to_string())?;
        info!("[autostart] enabled via .desktop file");
        Ok(())
    }
}

pub fn disable() -> Result<(), String> {
    #[cfg(target_os = "macos")]
    {
        let plist = plist_path().ok_or("cannot determine LaunchAgents path")?;
        if plist.exists() {
            std::fs::remove_file(&plist).map_err(|e| format!("remove plist: {}", e))?;
        }
        info!("[autostart] disabled");
        Ok(())
    }
    #[cfg(target_os = "windows")]
    {
        use winreg::enums::*;
        use winreg::RegKey;
        let hkcu = RegKey::predef(HKEY_CURRENT_USER);
        if let Ok(key) = hkcu.open_subkey_with_flags(
            "Software\\Microsoft\\Windows\\CurrentVersion\\Run",
            KEY_WRITE,
        ) {
            let _ = key.delete_value("OverseerDesktop");
        }
        info!("[autostart] disabled");
        Ok(())
    }
    #[cfg(target_os = "linux")]
    {
        let desktop_file = dirs::config_dir()
            .ok_or("cannot determine config dir")?
            .join("autostart")
            .join("overseer-desktop.desktop");
        if desktop_file.exists() {
            std::fs::remove_file(&desktop_file).map_err(|e| e.to_string())?;
        }
        info!("[autostart] disabled");
        Ok(())
    }
}
