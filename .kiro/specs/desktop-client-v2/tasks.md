# Implementation Tasks

## Task 1: Hide Console Window on Windows
- [x] Update `.github/workflows/release-desktop.yml` to add `-H windowsgui` to ldflags
- [x] Update build instructions in design doc
- Requirements: 1.1, 1.2, 1.3, 1.4

## Task 2: Multi-Language Support (locale package)
- [x] Create `cmd/desktop/internal/locale/locale.go` with `T(key)` function and OS language detection
- [x] Create `cmd/desktop/internal/locale/zh.go` with Chinese string map
- [x] Create `cmd/desktop/internal/locale/en.go` with English string map
- [x] Update tray menu to use `locale.T()` for all strings
- [x] Update setup dialog to use localized strings
- Requirements: 9.1, 9.2, 9.3, 9.4, 9.5, 9.6

## Task 3: Auto-Start on Boot
- [x] Create `cmd/desktop/internal/autostart/autostart.go` interface
- [x] Create `cmd/desktop/internal/autostart/autostart_windows.go` (registry)
- [x] Create `cmd/desktop/internal/autostart/autostart_darwin.go` (LaunchAgent)
- [x] Create `cmd/desktop/internal/autostart/autostart_linux.go` (XDG)
- [x] Add auto-start checkbox to tray menu
- Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.7

## Task 4: Auto Re-Register on 401
- [x] Modify `wsclient` to detect 401 and return typed error
- [x] Add callback/channel mechanism for auth failure notification
- [x] Update `main.go` to handle auth failure: clear config, re-show setup dialog
- Requirements: 3.1, 3.2, 3.3, 3.4, 3.5

## Task 5: Settings Dialog in Tray Menu
- [x] Create `cmd/desktop/internal/setup/settings_windows.go` (WPF dialog)
- [x] Create `cmd/desktop/internal/setup/settings_other.go` (terminal fallback)
- [x] Add "设置" menu item to tray
- [x] Implement save + reconnect logic
- Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 4.7

## Task 6: Rich Notifications
- [x] Extend `wsclient.PushEvent` with Channel, Source, Icon, Level fields
- [x] Update backend `broadcastPushEvent` to include level and icon in payload
- [x] Update notifier to map level to toast audio
- [x] Update notifier to show source/channel in notification body
- Requirements: 6.1, 6.2, 6.3, 6.4, 6.5, 6.6, 6.7, 6.8

## Task 7: Auto-Update Check
- [x] Create `cmd/desktop/internal/updater/updater.go`
- [x] Add `LastUpdateCheck` field to config
- [x] Call updater on startup in main.go
- [x] Show toast notification when new version available
- Requirements: 7.1, 7.2, 7.3, 7.4, 7.5, 7.6, 7.7

## Task 8: Local Notification History Cache
- [x] Create `cmd/desktop/internal/cache/cache.go` (SQLite)
- [x] Store notifications on receive in main.go
- [x] Add "最近通知" submenu to tray with last 5 items
- [x] Add "查看全部" item opening Web UI history
- Requirements: 8.1, 8.2, 8.3, 8.4, 8.5, 8.6, 8.7

## Task 9: Desktop Device Online Status (Backend + Web UI)
- [x] Extend `ws.Client` with DeviceKey field
- [x] Extend `ws.Hub` to track online device keys
- [x] Set DeviceKey in WSHandler when api_key auth succeeds
- [x] Create `GET /api/devices/status` handler
- [x] Update Web UI device list to show online/offline indicator
- Requirements: 5.1, 5.2, 5.3, 5.4, 5.5, 5.6
