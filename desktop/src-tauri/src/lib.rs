mod api;
mod cache;
mod config;
mod grouper;
mod locale;
mod tray;
mod updater;
mod websocket;

use cache::{Cache, NotificationEntry};
use chrono::Utc;
use config::AppConfig;
use grouper::Grouper;
use log::info;
use std::sync::Arc;
use tauri::{AppHandle, Emitter, Listener, Manager};
use tokio::sync::Mutex;
use tray::TrayManager;
use websocket::{PushEvent, WsClient};

/// App state shared across commands and event handlers.
struct AppState {
    config: Mutex<AppConfig>,
    cache: Arc<Cache>,
    tray_mgr: Arc<TrayManager>,
    ws_client: Mutex<Option<Arc<WsClient>>>,
    grouper: Mutex<Option<Arc<Grouper>>>,
}

// ─── Tauri Commands ───────────────────────────────────────────────────────────

#[tauri::command]
async fn get_config(state: tauri::State<'_, Arc<AppState>>) -> Result<AppConfig, String> {
    Ok(state.config.lock().await.clone())
}

#[tauri::command]
async fn save_config(
    state: tauri::State<'_, Arc<AppState>>,
    config: AppConfig,
) -> Result<(), String> {
    config.save()?;
    *state.config.lock().await = config;
    Ok(())
}

#[tauri::command]
async fn is_configured(state: tauri::State<'_, Arc<AppState>>) -> Result<bool, String> {
    Ok(state.config.lock().await.is_configured())
}

#[tauri::command]
async fn register_device(
    state: tauri::State<'_, Arc<AppState>>,
    server_url: String,
    token: String,
    device_name: String,
) -> Result<(), String> {
    let name = if device_name.is_empty() {
        let hostname = hostname::get()
            .map(|h| h.to_string_lossy().to_string())
            .unwrap_or_else(|_| "Desktop".to_string());
        format!("Desktop {}", hostname)
    } else {
        device_name
    };

    let device = api::register_device(&server_url, &name, &token).await?;

    let mut cfg = state.config.lock().await;
    cfg.server_url = server_url;
    cfg.api_key = device.device_key;
    cfg.device_id = device.id;
    cfg.device_name = device.name;
    cfg.save()?;

    Ok(())
}

#[tauri::command]
async fn get_messages(
    state: tauri::State<'_, Arc<AppState>>,
    limit: Option<u32>,
) -> Result<Vec<NotificationEntry>, String> {
    state.cache.recent(limit.unwrap_or(50))
}

#[tauri::command]
async fn get_unread_count(state: tauri::State<'_, Arc<AppState>>) -> Result<i32, String> {
    state.cache.unread_count()
}

#[tauri::command]
async fn mark_all_read(state: tauri::State<'_, Arc<AppState>>) -> Result<(), String> {
    state.cache.mark_all_read()?;
    state.tray_mgr.update_unread(0);
    Ok(())
}

#[tauri::command]
async fn mark_read(state: tauri::State<'_, Arc<AppState>>, id: String) -> Result<(), String> {
    state.cache.mark_read(&id)?;
    let count = state.cache.unread_count().unwrap_or(0);
    state.tray_mgr.update_unread(count);
    Ok(())
}

#[tauri::command]
async fn ack_message(state: tauri::State<'_, Arc<AppState>>, id: String) -> Result<(), String> {
    let cfg = state.config.lock().await;
    let server_url = cfg.server_url.clone();
    drop(cfg);

    api::ack_message(&server_url, &id).await?;
    state.cache.mark_read(&id)?;
    let count = state.cache.unread_count().unwrap_or(0);
    state.tray_mgr.update_unread(count);
    Ok(())
}

#[tauri::command]
async fn snooze_message(
    state: tauri::State<'_, Arc<AppState>>,
    id: String,
    duration: String,
) -> Result<(), String> {
    let cfg = state.config.lock().await;
    let server_url = cfg.server_url.clone();
    drop(cfg);

    api::snooze_message(&server_url, &id, &duration).await
}

#[tauri::command]
async fn reconnect_ws(
    state: tauri::State<'_, Arc<AppState>>,
) -> Result<(), String> {
    if let Some(ws) = state.ws_client.lock().await.as_ref() {
        ws.reconnect();
    }
    Ok(())
}

#[tauri::command]
async fn get_connection_state(
    state: tauri::State<'_, Arc<AppState>>,
) -> Result<bool, String> {
    if let Some(ws) = state.ws_client.lock().await.as_ref() {
        Ok(ws.is_connected().await)
    } else {
        Ok(false)
    }
}

#[tauri::command]
async fn set_muted(
    state: tauri::State<'_, Arc<AppState>>,
    muted: bool,
) -> Result<(), String> {
    state.tray_mgr.set_muted(muted);
    let mut cfg = state.config.lock().await;
    cfg.muted = Some(muted);
    cfg.save()?;
    Ok(())
}

#[tauri::command]
async fn is_muted(state: tauri::State<'_, Arc<AppState>>) -> Result<bool, String> {
    Ok(state.tray_mgr.is_muted())
}

// ─── App Entry Point ──────────────────────────────────────────────────────────

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    env_logger::Builder::from_env(env_logger::Env::default().default_filter_or("info")).init();

    info!(
        "[desktop] overseer-desktop starting... (lang={})",
        locale::current_lang()
    );

    let cfg = AppConfig::load();
    let cache = Arc::new(Cache::new().expect("failed to initialize cache"));
    let tray_mgr = Arc::new(TrayManager::new());

    // Restore muted state
    if let Some(muted) = cfg.muted {
        tray_mgr.set_muted(muted);
    }

    // Check initial unread count
    // On fresh start, clear stale unread from previous sessions
    if let Ok(count) = cache.unread_count() {
        if count > 0 {
            // Clear old unread messages from previous Go client sessions
            let _ = cache.mark_all_read();
            tray_mgr.update_unread(0);
        }
    }

    let app_state = Arc::new(AppState {
        config: Mutex::new(cfg.clone()),
        cache: cache.clone(),
        tray_mgr: tray_mgr.clone(),
        ws_client: Mutex::new(None),
        grouper: Mutex::new(None),
    });

    tauri::Builder::default()
        .plugin(tauri_plugin_opener::init())
        .plugin(tauri_plugin_notification::init())
        .plugin(tauri_plugin_autostart::init(
            tauri_plugin_autostart::MacosLauncher::LaunchAgent,
            Some(vec![]),
        ))
        .plugin(tauri_plugin_store::Builder::default().build())
        .manage(app_state.clone())
        .invoke_handler(tauri::generate_handler![
            get_config,
            save_config,
            is_configured,
            register_device,
            get_messages,
            get_unread_count,
            mark_all_read,
            mark_read,
            ack_message,
            snooze_message,
            reconnect_ws,
            get_connection_state,
            set_muted,
            is_muted,
        ])
        .setup(move |app| {
            // Prevent app from exiting when all windows are closed (tray app)
            // This is handled by not having any default windows and using on_window_event

            let app_handle = app.handle().clone();
            let state = app_state.clone();
            let tray_mgr_clone = tray_mgr.clone();

            // Setup system tray
            TrayManager::setup_tray(&app_handle, tray_mgr_clone.clone())
                .expect("failed to setup tray");

            // If not configured, show setup window
            if !cfg.is_configured() {
                let _setup_window = tauri::WebviewWindowBuilder::new(
                    app,
                    "setup",
                    tauri::WebviewUrl::App("index.html#/setup".into()),
                )
                .title("Overseer Desktop - 首次配置")
                .inner_size(450.0, 380.0)
                .resizable(false)
                .center()
                .build()
                .expect("failed to create setup window");
            } else {
                // Start WebSocket connection
                let app_handle2 = app_handle.clone();
                let state2 = state.clone();
                let cache2 = cache.clone();
                let tray_mgr2 = tray_mgr.clone();

                tauri::async_runtime::spawn(async move {
                    start_ws_connection(app_handle2, state2, cache2, tray_mgr2).await;
                });

                // Auto-update check
                let app_handle3 = app_handle.clone();
                let state3 = state.clone();
                tauri::async_runtime::spawn(async move {
                    check_for_updates(app_handle3, state3).await;
                });
            }

            // Listen for events from frontend
            let state_for_events = state.clone();
            let app_for_events = app_handle.clone();
            let cache_for_events = cache.clone();
            let tray_for_events = tray_mgr.clone();

            app_handle.listen("setup-complete", move |_event| {
                let state = state_for_events.clone();
                let app = app_for_events.clone();
                let cache = cache_for_events.clone();
                let tray = tray_for_events.clone();

                tauri::async_runtime::spawn(async move {
                    start_ws_connection(app.clone(), state.clone(), cache, tray).await;
                    check_for_updates(app, state).await;
                });
            });

            // Listen for open-webui event
            let state_webui = state.clone();
            app_handle.listen("open-webui", move |_| {
                let state = state_webui.clone();
                tauri::async_runtime::spawn(async move {
                    let cfg = state.config.lock().await;
                    let _ = open::that(&cfg.server_url);
                });
            });

            // Listen for reconnect event
            let state_reconnect = state.clone();
            app_handle.listen("reconnect", move |_| {
                let state = state_reconnect.clone();
                tauri::async_runtime::spawn(async move {
                    if let Some(ws) = state.ws_client.lock().await.as_ref() {
                        ws.reconnect();
                    }
                });
            });

            // Listen for open-settings event
            let app_settings = app_handle.clone();
            app_handle.listen("open-settings", move |_| {
                let _ = tauri::WebviewWindowBuilder::new(
                    &app_settings,
                    "settings",
                    tauri::WebviewUrl::App("index.html#/settings".into()),
                )
                .title("Overseer Desktop - 设置")
                .inner_size(450.0, 350.0)
                .resizable(false)
                .center()
                .build();
            });

            // Listen for open-messages event
            let app_messages = app_handle.clone();
            app_handle.listen("open-messages", move |_| {
                // Try to focus existing window or create new one
                if let Some(win) = app_messages.get_webview_window("messages") {
                    let _ = win.show();
                    let _ = win.set_focus();
                } else {
                    let _ = tauri::WebviewWindowBuilder::new(
                        &app_messages,
                        "messages",
                        tauri::WebviewUrl::App("index.html#/messages".into()),
                    )
                    .title("Overseer - 消息列表")
                    .inner_size(500.0, 600.0)
                    .center()
                    .build();
                }
            });

            // Listen for tray-click to open messages window
            let app_tray_click = app_handle.clone();
            app_handle.listen("tray-click", move |_| {
                if let Some(win) = app_tray_click.get_webview_window("messages") {
                    let _ = win.show();
                    let _ = win.set_focus();
                    // Emit refresh event so the page reloads data
                    let _ = win.emit("refresh-messages", ());
                } else {
                    let _ = tauri::WebviewWindowBuilder::new(
                        &app_tray_click,
                        "messages",
                        tauri::WebviewUrl::App("index.html#/messages".into()),
                    )
                    .title("Overseer - 消息列表")
                    .inner_size(500.0, 600.0)
                    .center()
                    .build();
                }
            });

            Ok(())
        })
        .on_window_event(|window, event| {
            // Hide window instead of closing to keep the tray app running
            if let tauri::WindowEvent::CloseRequested { api, .. } = event {
                let _ = window.hide();
                api.prevent_close();
            }
        })
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}

async fn start_ws_connection(
    app: AppHandle,
    state: Arc<AppState>,
    cache: Arc<Cache>,
    tray_mgr: Arc<TrayManager>,
) {
    let cfg = state.config.lock().await.clone();
    if !cfg.is_configured() {
        return;
    }

    // Create grouper
    let grouper = Arc::new(Grouper::new(app.clone()));
    *state.grouper.lock().await = Some(grouper.clone());

    // Create WebSocket client
    let ws = Arc::new(WsClient::new(
        app.clone(),
        cfg.server_url.clone(),
        cfg.api_key.clone(),
    ));
    *state.ws_client.lock().await = Some(ws.clone());

    // Listen for push events
    let cache_push = cache.clone();
    let tray_push = tray_mgr.clone();
    let grouper_push = grouper.clone();
    let app_push = app.clone();
    let muted_ref = tray_mgr.clone();

    app.listen("ws-push", move |event| {
        let payload = event.payload();
        if let Ok(push) = serde_json::from_str::<PushEvent>(payload) {
            // Check TTL expiry
            if !push.expires_at.is_empty() {
                if Cache::is_expired(&Some(push.expires_at.clone())) {
                    info!("[desktop] skipping expired message: {}", push.id);
                    return;
                }
            }

            // Store in cache
            let entry = NotificationEntry {
                id: push.id.clone(),
                title: push.title.clone(),
                body: push.body.clone(),
                url: push.url.clone(),
                channel: push.channel.clone(),
                source: push.source.clone(),
                level: push.level.clone(),
                received_at: Utc::now().to_rfc3339(),
                unread: true,
                acked_at: None,
                expires_at: if push.expires_at.is_empty() {
                    None
                } else {
                    Some(push.expires_at.clone())
                },
            };
            info!("[desktop] caching message id={} title={}", entry.id, entry.title);
            match cache_push.add(&entry) {
                Ok(_) => info!("[desktop] cached successfully"),
                Err(e) => info!("[desktop] cache add FAILED: {}", e),
            }

            // Update unread count
            let count = cache_push.unread_count().unwrap_or(0);
            info!("[desktop] unread count updated: {}", count);
            tray_push.update_unread(count);

            // Check muted
            if muted_ref.is_muted() {
                info!("[desktop] muted, skipping notification: {}", push.title);
                return;
            }

            // Route through grouper
            if let Some(event_to_show) = grouper_push.ingest(push) {
                let _ = app_push.emit("show-notification", &event_to_show);

                // Show native notification
                use tauri_plugin_notification::NotificationExt;
                let mut notification = app_push.notification()
                    .builder()
                    .title(&event_to_show.title);

                if !event_to_show.body.is_empty() {
                    notification = notification.body(&event_to_show.body);
                }

                let _ = notification.show();
                info!("[desktop] notification shown: {}", event_to_show.title);
            }
        }
    });

    // Listen for ack_sync events
    let cache_ack = cache.clone();
    let tray_ack = tray_mgr.clone();
    app.listen("ws-ack-sync", move |event| {
        let payload = event.payload();
        if let Ok(ack) = serde_json::from_str::<websocket::AckSyncEvent>(payload) {
            let _ = cache_ack.mark_ack_synced(&ack.message_id);
            let count = cache_ack.unread_count().unwrap_or(0);
            tray_ack.update_unread(count);
        }
    });

    // Listen for auth failure
    app.listen("ws-auth-fail", move |_| {
        info!("[desktop] auth failed, need re-registration");
        // Could emit event to show setup window
    });

    // Update connection state in tray
    let tray_state = tray_mgr.clone();
    app.listen("ws-state", move |event| {
        let payload = event.payload();
        let connected = payload.contains("connected") && !payload.contains("disconnected");
        tray_state.set_connected(connected);
    });
}

async fn check_for_updates(app: AppHandle, state: Arc<AppState>) {
    let cfg = state.config.lock().await.clone();
    let current_version = env!("CARGO_PKG_VERSION");

    if let Some(result) =
        updater::check_update(current_version, cfg.last_update_check.as_deref()).await
    {
        let _ = app.emit("update-available", &serde_json::json!({
            "version": result.version,
            "download_url": result.download_url,
        }));
    }

    // Update last check time
    let mut cfg = state.config.lock().await;
    cfg.last_update_check = Some(Utc::now().to_rfc3339());
    let _ = cfg.save();
}
