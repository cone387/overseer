# Technical Design: Desktop Client v2

## Overview

This design covers 9 features for the Overseer Desktop Client second iteration. The implementation is organized into independent packages that can be developed and tested in isolation.

## Architecture

```
cmd/desktop/
├── main.go                              # Entry point, orchestration
├── internal/
│   ├── config/config.go                 # JSON config + last_update_check field
│   ├── api/api.go                       # HTTP client (register)
│   ├── setup/                           # First-time & re-registration dialogs
│   ├── autostart/
│   │   ├── autostart.go                 # Interface
│   │   ├── autostart_windows.go         # Registry-based
│   │   ├── autostart_darwin.go          # LaunchAgent plist
│   │   └── autostart_linux.go           # XDG .desktop file
│   ├── locale/
│   │   ├── locale.go                    # Locale detection + string lookup
│   │   ├── zh.go                        # Chinese strings
│   │   └── en.go                        # English strings
│   ├── updater/updater.go              # GitHub Releases version check
│   ├── cache/cache.go                   # Local SQLite notification history
│   ├── wsclient/wsclient.go            # WebSocket client (add 401 detection)
│   ├── notifier/                        # Platform notifications (add rich fields)
│   └── tray/tray.go                     # Systray (add settings, autostart, history submenu)
```

## Design Decisions

### 1. Hide Console Window
- Add `-H windowsgui` to ldflags in GitHub Actions workflow and local build instructions
- No code changes needed, purely a build flag

### 2. Auto-Start
- New `autostart` package with platform-specific implementations
- Windows: `golang.org/x/sys/windows/registry` for registry access
- macOS: write XML plist to `~/Library/LaunchAgents/com.overseer.desktop.plist`
- Linux: write .desktop file to `~/.config/autostart/overseer-desktop.desktop`
- State persisted in OS (read back on startup to sync checkbox)

### 3. Auto Re-Register on 401
- `wsclient` returns a typed error `ErrUnauthorized` when HTTP 401 is received during dial
- `main.go` catches this error, clears config, re-shows setup dialog
- Uses a channel/callback pattern to signal the main goroutine

### 4. Settings Dialog
- Reuse PowerShell WPF approach (Windows) / terminal (other)
- New `setup/settings_windows.go` with editable fields
- Tray menu item triggers it

### 5. Online Status Tracking (Backend)
- Backend `ws.Hub` extended with a `deviceKeys` map tracking which desktop device_key is connected
- New API: `GET /api/devices/status` returns device list with `online` boolean
- Web UI fetches this endpoint and renders green/gray dots

### 6. Rich Notifications
- Extend `wsclient.PushEvent` with `Channel`, `Source`, `Icon`, `Level` fields
- Backend `broadcastPushEvent` already includes these (channel, source are present; add level, icon)
- Notifier maps level to `toast.Audio` constant
- Icon: download to temp file, pass absolute path to toast

### 7. Auto-Update Check
- New `updater` package
- Calls `https://api.github.com/repos/OWNER/REPO/releases/latest`
- Compares tag with `version` variable (set at build time)
- Stores `last_update_check` timestamp in config
- Shows toast notification with download link if newer version found

### 8. Local Notification History
- New `cache` package using `modernc.org/sqlite` (already a dependency)
- Single table: `notifications(id, title, body, url, channel, source, received_at)`
- Max 50 rows (DELETE oldest on INSERT when count > 50)
- Tray submenu shows last 5 titles, click opens URL

### 9. Multi-Language
- New `locale` package with `T(key)` function
- Detects OS language at startup (Windows: `GetUserDefaultUILanguage`, others: `LANG` env)
- Two string maps: `zh` and `en`
- All UI strings go through `locale.T("key")`

## Backend Changes (Requirement 5 only)

- Extend `ws.Hub` to track device_key per client
- Extend `ws.Client` with optional `DeviceKey` field
- `WSHandler.HandleWS` sets `client.DeviceKey` when api_key auth succeeds
- New handler: `GET /api/devices/status` queries Hub for online device keys, joins with devices table
- Web UI: add online indicator dot to device list component
