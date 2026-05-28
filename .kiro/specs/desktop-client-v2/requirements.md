# Requirements Document

## Introduction

Overseer Desktop Client v2 adds nine improvement features to the existing desktop notification client. The current client (`cmd/desktop/`) provides system tray integration, Windows Toast notifications, WebSocket connectivity with exponential backoff, first-time setup GUI, mute toggle, and GitHub Actions auto-release. This iteration enhances the client with hidden console window, auto-start on boot, automatic re-registration on key invalidation, a settings menu, backend device online status tracking, rich notifications, auto-update checking, local notification history cache, and multi-language support.

## Glossary

- **Desktop_Client**: The Overseer desktop notification application (`cmd/desktop/`) built in Go
- **Tray_Menu**: The system tray context menu displayed when the user interacts with the tray icon
- **WebSocket_Client**: The `wsclient` package that maintains a persistent WebSocket connection to the backend
- **Setup_Dialog**: The PowerShell WPF dialog (Windows) or terminal prompt (macOS/Linux) for first-time configuration
- **Backend**: The Overseer Gin-based HTTP/WebSocket server
- **Config_Store**: The local JSON configuration file at platform-specific paths
- **Notification_Cache**: A local SQLite database storing recent notification history
- **Notifier**: The platform-specific notification display component (`notifier` package)
- **Auto_Start_Manager**: The component responsible for registering/unregistering the application for OS boot startup
- **Update_Checker**: The component that queries GitHub Releases API for newer versions
- **Locale_Manager**: The component that detects system language and provides localized strings
- **Online_Status_Tracker**: The backend component that tracks active WebSocket connections from desktop devices
- **Channel_Level**: The notification urgency level associated with a channel (critical, timeSensitive, default, passive)

## Requirements

### Requirement 1: Hide Console Window on Windows

**User Story:** As a Windows user, I want the desktop client to run as a pure GUI application, so that double-clicking the exe does not show a black terminal window.

#### Acceptance Criteria

1. WHEN the Desktop_Client is built for Windows, THE Build_System SHALL include `-ldflags="-H windowsgui"` in the linker flags
2. WHEN a Windows user double-clicks the executable, THE Desktop_Client SHALL start without displaying a console window
3. WHILE the Desktop_Client is running on Windows, THE Desktop_Client SHALL operate entirely through the system tray without any visible console
4. WHEN the Desktop_Client is built for macOS or Linux, THE Build_System SHALL omit the `-H windowsgui` flag

### Requirement 2: Auto-Start on Boot

**User Story:** As a user, I want the desktop client to start automatically when I log in, so that I receive notifications without manually launching the application.

#### Acceptance Criteria

1. WHEN the user enables auto-start from the Tray_Menu, THE Auto_Start_Manager SHALL register the Desktop_Client for automatic startup on the current operating system
2. WHEN the user disables auto-start from the Tray_Menu, THE Auto_Start_Manager SHALL remove the Desktop_Client from automatic startup registration
3. WHEN auto-start is enabled on Windows, THE Auto_Start_Manager SHALL write an entry to the registry key `HKCU\Software\Microsoft\Windows\CurrentVersion\Run` with the full path to the executable
4. WHEN auto-start is enabled on macOS, THE Auto_Start_Manager SHALL create a LaunchAgent plist file in `~/Library/LaunchAgents/` with the correct executable path
5. WHEN auto-start is enabled on Linux, THE Auto_Start_Manager SHALL create a `.desktop` file in `~/.config/autostart/` conforming to the XDG Autostart specification
6. THE Tray_Menu SHALL display the current auto-start state as a checkable menu item
7. WHEN the Desktop_Client starts, THE Auto_Start_Manager SHALL read the current auto-start registration state from the operating system and reflect it in the Tray_Menu

### Requirement 3: Auto Re-Register on API Key Invalidation

**User Story:** As a user, I want the desktop client to automatically prompt me to re-register when my API key becomes invalid, so that I can restore connectivity without manually deleting the config file.

#### Acceptance Criteria

1. WHEN the WebSocket_Client receives a 401 HTTP status during connection, THE Desktop_Client SHALL clear the stored api_key from the Config_Store
2. WHEN the api_key is cleared due to a 401 response, THE Desktop_Client SHALL display the Setup_Dialog to allow the user to re-register
3. WHEN the user completes re-registration through the Setup_Dialog, THE Desktop_Client SHALL save the new credentials to the Config_Store and establish a new WebSocket connection
4. IF the user cancels the Setup_Dialog during re-registration, THEN THE Desktop_Client SHALL exit gracefully
5. WHILE the re-registration dialog is displayed, THE Desktop_Client SHALL stop WebSocket reconnection attempts

### Requirement 4: Settings Entry in Tray Menu

**User Story:** As a user, I want a settings option in the tray menu, so that I can change the server URL, re-register, or reset configuration without editing files manually.

#### Acceptance Criteria

1. THE Tray_Menu SHALL include a "设置" (Settings) menu item
2. WHEN the user clicks the "设置" menu item, THE Desktop_Client SHALL display a settings dialog
3. THE settings dialog SHALL allow the user to view and modify the current server URL
4. THE settings dialog SHALL provide a button to trigger re-registration with a new token
5. THE settings dialog SHALL provide a button to reset all configuration and restart the setup flow
6. WHEN the user confirms server URL changes in the settings dialog, THE Desktop_Client SHALL save the updated URL to the Config_Store and reconnect the WebSocket_Client
7. IF the user cancels the settings dialog, THEN THE Desktop_Client SHALL discard all changes and return to normal operation

### Requirement 5: Desktop Device Online Status in Web UI

**User Story:** As a user viewing the Web UI, I want to see which desktop devices are currently online, so that I can verify my devices are connected and receiving notifications.

#### Acceptance Criteria

1. WHEN a desktop device establishes a WebSocket connection, THE Online_Status_Tracker SHALL record that device as online
2. WHEN a desktop device's WebSocket connection closes, THE Online_Status_Tracker SHALL record that device as offline
3. THE Backend SHALL provide an API endpoint that returns the online/offline status of all desktop devices
4. WHEN the Web UI device list is displayed, THE Web_UI SHALL show a green dot indicator next to online desktop devices
5. WHEN the Web UI device list is displayed, THE Web_UI SHALL show a gray dot indicator next to offline desktop devices
6. THE online status API response SHALL include the device ID, device name, device type, and online/offline boolean for each device

### Requirement 6: Rich Notifications

**User Story:** As a user, I want notifications to display channel name, source information, and play different sounds based on urgency, so that I can quickly assess notification importance without reading the full content.

#### Acceptance Criteria

1. WHEN a push event is received, THE Notifier SHALL display the channel name as part of the notification content
2. WHEN a push event is received, THE Notifier SHALL display the source as attribution text in the notification
3. WHEN a push event contains an icon URL, THE Notifier SHALL download and display the icon in the notification
4. WHEN the Channel_Level is "critical", THE Notifier SHALL play an alarm audio sound with the notification
5. WHEN the Channel_Level is "timeSensitive", THE Notifier SHALL play a reminder audio sound with the notification
6. WHEN the Channel_Level is "default", THE Notifier SHALL play the default system notification sound
7. WHEN the Channel_Level is "passive", THE Notifier SHALL display the notification silently without audio
8. THE WebSocket push event payload SHALL include channel name, source, icon URL, and Channel_Level fields

### Requirement 7: Auto-Update Check

**User Story:** As a user, I want the desktop client to notify me when a newer version is available, so that I can keep the application up to date with the latest features and fixes.

#### Acceptance Criteria

1. WHEN the Desktop_Client starts, THE Update_Checker SHALL check the GitHub Releases API for a version newer than the current running version
2. WHEN a newer version is found, THE Update_Checker SHALL display a toast notification containing the new version number and a download link to the GitHub release page
3. THE Update_Checker SHALL check for updates at most once per 24-hour period
4. WHEN the last update check occurred less than 24 hours ago, THE Update_Checker SHALL skip the check
5. IF the GitHub Releases API is unreachable, THEN THE Update_Checker SHALL log the error and continue normal operation without displaying any notification to the user
6. THE Update_Checker SHALL compare versions using semantic versioning rules
7. THE Update_Checker SHALL store the timestamp of the last successful check in the Config_Store

### Requirement 8: Local Notification History Cache

**User Story:** As a user, I want to see my recent notifications from the tray menu, so that I can quickly review missed notifications without opening the Web UI.

#### Acceptance Criteria

1. WHEN a push notification is received, THE Notification_Cache SHALL store the notification in a local SQLite database
2. THE Notification_Cache SHALL retain a maximum of 50 notifications, removing the oldest entries when the limit is exceeded
3. THE Tray_Menu SHALL include a "最近通知" (Recent Notifications) submenu displaying the 5 most recent notifications
4. WHEN a notification entry in the submenu is clicked, THE Desktop_Client SHALL open the notification URL in the default browser
5. THE "最近通知" submenu SHALL include a "查看全部" (View All) item that opens the Web UI history page in the default browser
6. WHEN the Desktop_Client starts, THE Notification_Cache SHALL load existing cached notifications from the SQLite database
7. THE notification cache entry SHALL store the notification title, body, URL, channel name, source, and received timestamp

### Requirement 9: Multi-Language Support

**User Story:** As a user, I want the desktop client to display in my system language, so that I can understand all menu items and dialog text regardless of my language preference.

#### Acceptance Criteria

1. WHEN the Desktop_Client starts, THE Locale_Manager SHALL detect the operating system's current language setting
2. THE Locale_Manager SHALL support Chinese (zh) and English (en) language packs
3. WHEN the system language is Chinese, THE Locale_Manager SHALL use Chinese strings for all UI text
4. WHEN the system language is not Chinese, THE Locale_Manager SHALL fall back to English strings for all UI text
5. THE Tray_Menu items, Setup_Dialog labels, settings dialog labels, and notification submenu text SHALL use strings provided by the Locale_Manager
6. THE Locale_Manager SHALL provide localized strings without requiring application restart when the language pack is loaded at startup
