mod api;
mod autostart;
mod cache;
mod config;
mod grouper;
mod locale;
mod updater;
mod wsclient;

use cache::Cache;
use chrono::Utc;
use config::Config;
use grouper::{Grouper, PushEvent};
use log::info;
use std::sync::Arc;
use tauri::{
    menu::{MenuBuilder, MenuItemBuilder},
    tray::TrayIconBuilder,
    AppHandle, Emitter, Manager, WindowEvent,
};
use tokio::sync::{mpsc, Mutex};
use wsclient::{WsClient, WsClientEvent};

/// Shared application state.
pub struct AppState {
    pub config: Arc<Mutex<Config>>,
    pub cache: Arc<Option<Cache>>,
    pub ws_connected: Arc<Mutex<bool>>,
    pub muted: Arc<Mutex<bool>>,
}

// ─── Tauri Commands ───────────────────────────────────────────────────────────

/// Register device with the server (called from frontend setup page).
#[tauri::command]
async fn register_device(
    app_handle: AppHandle,
    server_url: String,
    token: String,
    device_name: String,
    state: tauri::State<'_, AppState>,
) -> Result<String, String> {
    let name = if device_name.is_empty() {
        format!("Desktop {}", hostname::get().unwrap_or_default().to_string_lossy())
    } else {
        device_name
    };

    let device = api::register(&server_url, &name, &token).await?;

    let mut cfg = state.config.lock().await;
    cfg.server_url = server_url;
    cfg.api_key = device.device_key.clone();
    cfg.device_id = device.id.clone();
    cfg.device_name = device.name.clone();
    cfg.save()?;

    // Auto-start WS connection after registration
    let cfg_clone = cfg.clone();
    drop(cfg);
    start_ws_client(app_handle, cfg_clone);

    Ok(device.name)
}

/// Get current config state (for frontend to check if setup is needed).
#[tauri::command]
async fn get_config(state: tauri::State<'_, AppState>) -> Result<Config, String> {
    let cfg = state.config.lock().await;
    Ok(cfg.clone())
}

/// Get recent notifications from cache.
#[tauri::command]
async fn get_recent_notifications(
    count: Option<i32>,
    state: tauri::State<'_, AppState>,
) -> Result<Vec<cache::Entry>, String> {
    match state.cache.as_ref() {
        Some(c) => c.recent(count.unwrap_or(20)),
        None => Ok(vec![]),
    }
}

/// Get unread count.
#[tauri::command]
async fn get_unread_count(state: tauri::State<'_, AppState>) -> Result<i32, String> {
    match state.cache.as_ref() {
        Some(c) => c.unread_count(),
        None => Ok(0),
    }
}

/// Mark all notifications as read.
#[tauri::command]
async fn mark_all_read(state: tauri::State<'_, AppState>) -> Result<(), String> {
    if let Some(c) = state.cache.as_ref() {
        c.mark_all_read()?;
    }
    Ok(())
}

/// Check if WebSocket is connected.
#[tauri::command]
async fn is_connected(state: tauri::State<'_, AppState>) -> Result<bool, String> {
    Ok(*state.ws_connected.lock().await)
}

/// Toggle mute state.
#[tauri::command]
async fn toggle_mute(state: tauri::State<'_, AppState>) -> Result<bool, String> {
    let mut muted = state.muted.lock().await;
    *muted = !*muted;
    Ok(*muted)
}

/// Acknowledge a message.
#[tauri::command]
async fn ack_message(message_id: String, state: tauri::State<'_, AppState>) -> Result<(), String> {
    let cfg = state.config.lock().await;
    let base_url = cfg.server_url.clone();
    drop(cfg);
    api::ack_message(&base_url, &message_id).await?;
    if let Some(ref c) = *state.cache {
        let _ = c.mark_read(&message_id);
    }
    Ok(())
}

/// Update server URL in config.
#[tauri::command]
async fn update_server_url(
    app_handle: AppHandle,
    server_url: String,
    state: tauri::State<'_, AppState>,
) -> Result<(), String> {
    let mut cfg = state.config.lock().await;
    cfg.server_url = server_url;
    cfg.save()?;
    // Restart WS with new URL
    let cfg_clone = cfg.clone();
    drop(cfg);
    start_ws_client(app_handle, cfg_clone);
    Ok(())
}

/// Get autostart status.
#[tauri::command]
fn get_autostart_enabled() -> bool {
    autostart::is_enabled()
}

/// Toggle autostart.
#[tauri::command]
fn set_autostart(enabled: bool) -> Result<(), String> {
    if enabled {
        autostart::enable()
    } else {
        autostart::disable()
    }
}

// ─── App Setup ────────────────────────────────────────────────────────────────

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    env_logger::Builder::from_env(env_logger::Env::default().default_filter_or("info")).init();

    let cfg = Config::load();
    let notification_cache = Cache::new().ok();

    let state = AppState {
        config: Arc::new(Mutex::new(cfg.clone())),
        cache: Arc::new(notification_cache),
        ws_connected: Arc::new(Mutex::new(false)),
        muted: Arc::new(Mutex::new(false)),
    };

    tauri::Builder::default()
        .plugin(tauri_plugin_opener::init())
        .plugin(tauri_plugin_notification::init())
        .manage(state)
        .invoke_handler(tauri::generate_handler![
            register_device,
            get_config,
            get_recent_notifications,
            get_unread_count,
            mark_all_read,
            is_connected,
            toggle_mute,
            ack_message,
            update_server_url,
            get_autostart_enabled,
            set_autostart,
        ])
        .on_window_event(|window, event| {
            // Hide window instead of closing — keep running in tray
            if let WindowEvent::CloseRequested { api, .. } = event {
                window.hide().unwrap_or_default();
                api.prevent_close();
            }
        })
        .setup(|app| {
            let handle = app.handle().clone();

            // Build tray menu
            let quit = MenuItemBuilder::with_id("quit", locale::t("tray.quit")).build(app)?;
            let open_ui = MenuItemBuilder::with_id("open_ui", locale::t("tray.open_webui")).build(app)?;
            let reconnect = MenuItemBuilder::with_id("reconnect", locale::t("tray.reconnect")).build(app)?;
            let mute = MenuItemBuilder::with_id("mute", locale::t("tray.mute")).build(app)?;
            let show_window = MenuItemBuilder::with_id("show_window", "显示窗口").build(app)?;

            let menu = MenuBuilder::new(app)
                .item(&show_window)
                .item(&open_ui)
                .separator()
                .item(&mute)
                .item(&reconnect)
                .separator()
                .item(&quit)
                .build()?;

            let _tray = TrayIconBuilder::with_id("main")
                .tooltip("Overseer Desktop")
                .icon(tauri::image::Image::from_bytes(include_bytes!("../icons/32x32.png")).expect("load tray icon"))
                .icon_as_template(false)
                .menu(&menu)
                .on_menu_event(move |app, event| {
                    match event.id().as_ref() {
                        "quit" => {
                            app.exit(0);
                        }
                        "show_window" => {
                            if let Some(window) = app.get_webview_window("main") {
                                let _ = window.show();
                                let _ = window.set_focus();
                            }
                        }
                        "open_ui" => {
                            let state = app.state::<AppState>();
                            let cfg = state.config.blocking_lock();
                            if !cfg.server_url.is_empty() {
                                let _ = open::that(&cfg.server_url);
                            }
                        }
                        "reconnect" => {
                            let _ = app.emit("ws-reconnect", ());
                        }
                        "mute" => {
                            let state = app.state::<AppState>();
                            let mut muted = state.muted.blocking_lock();
                            *muted = !*muted;
                            info!("[tray] mute toggled: {}", *muted);
                        }
                        _ => {}
                    }
                })
                .on_tray_icon_event(|tray, event| {
                    match event {
                        tauri::tray::TrayIconEvent::Click { button, .. } => {
                            if button == tauri::tray::MouseButton::Left {
                                let app = tray.app_handle();
                                if let Some(window) = app.get_webview_window("main") {
                                    let _ = window.show();
                                    let _ = window.set_focus();
                                }
                            }
                        }
                        _ => {}
                    }
                })
                .menu_on_left_click(false)
                .build(app)?;

            // Start WebSocket connection if already configured
            if cfg.is_configured() {
                start_ws_client(handle, cfg);
            }

            Ok(())
        })
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}

/// Start the WebSocket client in a background task.
fn start_ws_client(handle: AppHandle, cfg: Config) {
    let (event_tx, mut event_rx) = mpsc::unbounded_channel::<WsClientEvent>();

    let ws_client = Arc::new(WsClient::new(
        cfg.server_url.clone(),
        cfg.api_key.clone(),
        event_tx,
    ));

    let state = handle.state::<AppState>();
    let cache = state.cache.clone();
    let muted = state.muted.clone();
    let ws_connected = state.ws_connected.clone();
    let config = state.config.clone();

    // Create grouper with notification callback
    let handle_clone = handle.clone();
    let muted_clone = muted.clone();
    let show_fn: grouper::ShowFn = Arc::new(move |title, body, _url, channel, _source, _level, _msg_id| {
        if let Ok(is_muted) = muted_clone.try_lock() {
            if *is_muted {
                return;
            }
        }

        // Emit to frontend
        let _ = handle_clone.emit("notification", serde_json::json!({
            "title": title,
            "body": body,
            "channel": channel,
        }));

        // Show system notification
        let enriched_body = if !channel.is_empty() {
            format!("[{}] {}", channel, body)
        } else {
            body
        };

        // Use tauri notification plugin
        if let Some(webview) = handle_clone.webview_windows().values().next() {
            let _ = tauri_plugin_notification::NotificationExt::notification(webview.app_handle())
                .builder()
                .title(&title)
                .body(&enriched_body)
                .show();
        }
    });

    let grouper = Arc::new(Grouper::new(show_fn));

    // Spawn WS client
    let ws_client_clone = ws_client.clone();
    tauri::async_runtime::spawn(async move {
        ws_client_clone.run().await;
    });

    // Spawn event handler
    let handle_events = handle.clone();
    tauri::async_runtime::spawn(async move {
        while let Some(event) = event_rx.recv().await {
            match event {
                WsClientEvent::Push(push) => {
                    info!("[ws] notification: [{}] {} - {}", push.channel, push.title, push.body);

                    // Cache the notification
                    if let Some(ref c) = *cache {
                        let expires_at = if push.expires_at.is_empty() {
                            None
                        } else {
                            chrono::DateTime::parse_from_rfc3339(&push.expires_at)
                                .ok()
                                .map(|dt| dt.with_timezone(&Utc))
                        };

                        let _ = c.add(&cache::Entry {
                            id: push.id.clone(),
                            title: push.title.clone(),
                            body: push.body.clone(),
                            url: push.url.clone(),
                            channel: push.channel.clone(),
                            source: push.source.clone(),
                            received_at: Utc::now(),
                            unread: true,
                            acked_at: None,
                            expires_at,
                        });
                    }

                    // Emit unread count update + update tray
                    if let Some(ref c) = *cache {
                        if let Ok(count) = c.unread_count() {
                            let _ = handle_events.emit("unread-count", count);
                            update_tray_tooltip(&handle_events, count);
                        }
                    }

                    // Route through grouper
                    grouper.ingest(PushEvent {
                        id: push.id,
                        source: push.source,
                        channel: push.channel,
                        title: push.title,
                        body: push.body,
                        url: push.url,
                        level: push.level,
                    });
                }
                WsClientEvent::AckSync { message_id } => {
                    if let Some(ref c) = *cache {
                        let _ = c.mark_ack_synced(&message_id);
                        if let Ok(count) = c.unread_count() {
                            let _ = handle_events.emit("unread-count", count);
                            update_tray_tooltip(&handle_events, count);
                        }
                    }
                }
                WsClientEvent::Connected => {
                    *ws_connected.lock().await = true;
                    let _ = handle_events.emit("ws-status", "connected");
                }
                WsClientEvent::Disconnected => {
                    *ws_connected.lock().await = false;
                    let _ = handle_events.emit("ws-status", "disconnected");
                }
                WsClientEvent::AuthFailed => {
                    *ws_connected.lock().await = false;
                    let mut cfg = config.lock().await;
                    cfg.api_key.clear();
                    let _ = cfg.save();
                    let _ = handle_events.emit("ws-status", "auth_failed");
                }
            }
        }
    });

    // Check for updates in background
    let handle_updater = handle.clone();
    let config_updater = state.config.clone();
    tauri::async_runtime::spawn(async move {
        let cfg = config_updater.lock().await;
        let last_check = cfg.last_update_check;
        drop(cfg);

        if let Some(result) = updater::check("dev", last_check).await {
            if result.available {
                let _ = handle_updater.emit("update-available", serde_json::json!({
                    "version": result.version,
                    "url": result.download_url,
                }));
            }
        }

        // Update last check time
        let mut cfg = config_updater.lock().await;
        cfg.last_update_check = Some(Utc::now());
        let _ = cfg.save();
    });
}

/// Update tray icon tooltip to show unread count.
fn update_tray_tooltip(handle: &AppHandle, unread_count: i32) {
    if let Some(tray) = handle.tray_by_id("main") {
        let tooltip = if unread_count > 0 {
            format!("Overseer Desktop ({} 未读)", unread_count)
        } else {
            "Overseer Desktop".to_string()
        };
        let _ = tray.set_tooltip(Some(&tooltip));
        // On macOS, set title to show badge number next to tray icon
        #[cfg(target_os = "macos")]
        {
            let title = if unread_count > 0 {
                Some(format!("{}", unread_count))
            } else {
                None
            };
            let _ = tray.set_title(title.as_deref());
        }
    }
}
