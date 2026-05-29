use crate::locale::t;
use log::info;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use tauri::{
    image::Image,
    menu::{CheckMenuItem, Menu, MenuItem, PredefinedMenuItem, Submenu},
    tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent},
    AppHandle, Emitter,
};

pub struct TrayManager {
    flashing: Arc<AtomicBool>,
    muted: Arc<AtomicBool>,
    connected: Arc<AtomicBool>,
    unread_count: Arc<std::sync::atomic::AtomicI32>,
}

impl TrayManager {
    pub fn new() -> Self {
        Self {
            flashing: Arc::new(AtomicBool::new(false)),
            muted: Arc::new(AtomicBool::new(false)),
            connected: Arc::new(AtomicBool::new(false)),
            unread_count: Arc::new(std::sync::atomic::AtomicI32::new(0)),
        }
    }

    pub fn is_muted(&self) -> bool {
        self.muted.load(Ordering::Relaxed)
    }

    pub fn set_muted(&self, muted: bool) {
        self.muted.store(muted, Ordering::Relaxed);
    }

    pub fn set_connected(&self, connected: bool) {
        self.connected.store(connected, Ordering::Relaxed);
    }

    pub fn update_unread(&self, count: i32) {
        self.unread_count.store(count, Ordering::Relaxed);
        self.flashing.store(count > 0, Ordering::Relaxed);
    }

    pub fn setup_tray(app: &AppHandle, tray_mgr: Arc<TrayManager>) -> Result<(), String> {
        let menu = Self::build_menu(app)?;

        let icon = Image::from_bytes(include_bytes!("../icons/icon.png"))
            .expect("failed to load embedded icon");

        let tray_mgr_for_menu = tray_mgr.clone();

        let tray = TrayIconBuilder::with_id("main-tray")
            .icon(icon)
            .menu(&menu)
            .menu_on_left_click(false)
            .tooltip("Overseer Desktop")
            .on_menu_event(move |app, event| {
                Self::handle_menu_event(app, &event.id().0, &tray_mgr_for_menu);
            })
            .on_tray_icon_event(|tray, event| {
                if let TrayIconEvent::Click {
                    button: MouseButton::Left,
                    button_state: MouseButtonState::Up,
                    ..
                } = event
                {
                    let _ = tray.app_handle().emit("tray-click", ());
                }
            })
            .build(app)
            .map_err(|e| format!("build tray: {}", e))?;

        // Start flash loop in a separate thread
        let flashing = tray_mgr.flashing.clone();
        std::thread::spawn(move || {
            let mut visible = true;
            loop {
                std::thread::sleep(std::time::Duration::from_millis(500));

                if flashing.load(Ordering::Relaxed) {
                    visible = !visible;
                    let _ = tray.set_visible(visible);
                } else {
                    if !visible {
                        let _ = tray.set_visible(true);
                        visible = true;
                    }
                }
            }
        });

        Ok(())
    }

    fn build_menu(app: &AppHandle) -> Result<Menu<tauri::Wry>, String> {
        let menu = Menu::new(app).map_err(|e| e.to_string())?;

        // Status (disabled)
        let status = MenuItem::with_id(app, "status", t("tray.connected"), false, None::<&str>)
            .map_err(|e| e.to_string())?;
        menu.append(&status).map_err(|e| e.to_string())?;

        // Separator
        let sep = PredefinedMenuItem::separator(app).map_err(|e| e.to_string())?;
        menu.append(&sep).map_err(|e| e.to_string())?;

        // Messages window
        let messages = MenuItem::with_id(app, "messages", t("tray.messages"), true, None::<&str>)
            .map_err(|e| e.to_string())?;
        menu.append(&messages).map_err(|e| e.to_string())?;

        // Recent notifications submenu
        let recent_sub = Submenu::with_id(app, "recent", t("tray.recent"), true)
            .map_err(|e| e.to_string())?;
        let no_history =
            MenuItem::with_id(app, "no_history", t("tray.no_history"), false, None::<&str>)
                .map_err(|e| e.to_string())?;
        recent_sub.append(&no_history).map_err(|e| e.to_string())?;
        menu.append(&recent_sub).map_err(|e| e.to_string())?;

        // Separator
        let sep2 = PredefinedMenuItem::separator(app).map_err(|e| e.to_string())?;
        menu.append(&sep2).map_err(|e| e.to_string())?;

        // Mute (checkbox)
        let mute = CheckMenuItem::with_id(app, "mute", t("tray.mute"), true, false, None::<&str>)
            .map_err(|e| e.to_string())?;
        menu.append(&mute).map_err(|e| e.to_string())?;

        // Autostart (checkbox)
        let autostart =
            CheckMenuItem::with_id(app, "autostart", t("tray.autostart"), true, false, None::<&str>)
                .map_err(|e| e.to_string())?;
        menu.append(&autostart).map_err(|e| e.to_string())?;

        // Settings
        let settings =
            MenuItem::with_id(app, "settings", t("tray.settings"), true, None::<&str>)
                .map_err(|e| e.to_string())?;
        menu.append(&settings).map_err(|e| e.to_string())?;

        // Open Web UI
        let open_webui =
            MenuItem::with_id(app, "open_webui", t("tray.open_webui"), true, None::<&str>)
                .map_err(|e| e.to_string())?;
        menu.append(&open_webui).map_err(|e| e.to_string())?;

        // Reconnect
        let reconnect =
            MenuItem::with_id(app, "reconnect", t("tray.reconnect"), true, None::<&str>)
                .map_err(|e| e.to_string())?;
        menu.append(&reconnect).map_err(|e| e.to_string())?;

        // Separator
        let sep3 = PredefinedMenuItem::separator(app).map_err(|e| e.to_string())?;
        menu.append(&sep3).map_err(|e| e.to_string())?;

        // Quit
        let quit = MenuItem::with_id(app, "quit", t("tray.quit"), true, None::<&str>)
            .map_err(|e| e.to_string())?;
        menu.append(&quit).map_err(|e| e.to_string())?;

        Ok(menu)
    }

    fn handle_menu_event(app: &AppHandle, id: &str, tray_mgr: &Arc<TrayManager>) {
        match id {
            "mute" => {
                let currently_muted = tray_mgr.is_muted();
                tray_mgr.set_muted(!currently_muted);
                let _ = app.emit("mute-changed", !currently_muted);
                info!(
                    "[tray] {}",
                    if !currently_muted {
                        t("status.muted")
                    } else {
                        t("status.unmuted")
                    }
                );
            }
            "autostart" => {
                let _ = app.emit("autostart-toggle", ());
            }
            "settings" => {
                let _ = app.emit("open-settings", ());
            }
            "open_webui" => {
                let _ = app.emit("open-webui", ());
            }
            "reconnect" => {
                let _ = app.emit("reconnect", ());
            }
            "messages" => {
                let _ = app.emit("open-messages", ());
            }
            "quit" => {
                info!("[tray] quit requested");
                app.exit(0);
            }
            _ => {}
        }
    }
}
