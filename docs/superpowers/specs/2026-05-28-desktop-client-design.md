# Overseer Desktop Client Design

## Overview

Build a lightweight, persistent desktop notification receiver for Windows and macOS. The client runs in the system tray, receives real-time push notifications from the Overseer backend via WebSocket, and displays native OS-level notifications. For advanced features (history, settings, channel management), users click through to the existing Web UI.

## Goals

- Pure Go, no WebView, minimal resource usage.
- Cross-platform: Windows (primary) and macOS (code-compatible).
- Auto-register with the Overseer backend on first launch.
- Auto-reconnect WebSocket with exponential backoff.
- Native system notifications with click-to-open-Web-UI.

## Non-Goals

- No built-in message history viewer (redirect to Web UI).
- No channel/settings management inside the desktop app.
- No standalone UI windows beyond the system tray menu.

## Section 1: Backend Changes (Minimal Intrusion)

### Database Migration

Add `006_desktop_devices.go` to extend the `devices` table:

```sql
ALTER TABLE devices ADD COLUMN type TEXT NOT NULL DEFAULT 'bark';
CREATE INDEX IF NOT EXISTS idx_devices_type_key ON devices(type, device_key);
```

Existing Bark devices retain `type = 'bark'` by default. The migration is idempotent (checks if column exists before ALTER).

### Model Update

Extend `model.Device` with a `Type` field:

```go
type Device struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    DeviceKey string    `json:"device_key"`
    Type      string    `json:"type"`       // "bark" | "desktop"
    IsDefault bool      `json:"is_default"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

### New API: Desktop Self-Registration

`POST /api/devices/desktop-register`

**Request body:**

```json
{
  "name": "MacBook Pro",
  "token": "your-register-token-from-config"
}
```

`name` is optional (defaults to "Desktop Client"). `token` is **required** and must match the `desktop.register_token` in config.yaml.

**Response:**

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "id": "uuid",
    "name": "MacBook Pro",
    "device_key": "dk_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
    "type": "desktop",
    "is_default": false,
    "created_at": "...",
    "updated_at": "..."
  }
}
```

- `device_key` is a long-lived API key (`dk_` prefix + 32 random hex characters).
- The endpoint requires a valid `token` matching `desktop.register_token` from config.yaml. If `register_token` is empty, registration is disabled (403).
- A `max_devices` limit can be configured to prevent abuse.
- Implementation note: register this route **outside** the authenticated API group (e.g., directly on the Gin engine).

### WebSocket Authentication Extension

The existing `/ws` endpoint validates JWT via query param or cookie. Extend it to also accept `?api_key=`:

1. If `api_key` query param is present:
   - Look up `devices` table where `device_key = ? AND type = 'desktop'`.
   - If found, allow the connection.
   - If not found, return 401.
2. Otherwise, fall through to the existing JWT/cookie validation (for Web UI compatibility).

The WebSocket Hub broadcasts `push` events to **all** connected clients. No pipeline changes are required because `broadcastPushEvent` already broadcasts to every active WebSocket connection.

## Section 2: Desktop Client Architecture

Directory: `desktop/` (or `cmd/desktop/`).

### Packages

- **`config`**
  - Reads/writes local JSON config.
  - Windows path: `%LOCALAPPDATA%/overseer-desktop/config.json`
  - macOS path: `~/.config/overseer-desktop/config.json`
  - Content:
    - `server_url`: full base URL of the Overseer backend, e.g. `http://localhost:8080` or `https://overseer.example.com`.
    - `api_key`: the long-lived desktop device key received from registration.
    - `device_id`: the UUID returned by the registration endpoint.
    - `device_name`: friendly name shown in the tray menu.

- **`api`**
  - `Register(baseURL, name) (*Device, error)` — calls `POST /api/devices/desktop-register`.
  - `ConnectWS(baseURL, apiKey) (*websocket.Conn, error)` — dials `/ws?api_key=xxx`.

- **`notifier`** (platform-specific build tags)
  - **Windows** (`//go:build windows`): uses `github.com/go-toast/toast` for native Windows Toast notifications. Supports click callbacks that open the browser.
  - **macOS** (`//go:build darwin`): uses `osascript display notification` via `os/exec`. Zero CGO dependencies.

- **`tray`**
  - Uses `github.com/getlantern/systray`.
  - Tray menu items:
    - Status label (non-clickable): "Connected" / "Disconnected"
    - "Open Web UI" — opens browser to `server_url`
    - "Reconnect" — force WebSocket reconnect
    - "Quit" — exit application

- **`wsclient`**
  - Manages the WebSocket lifecycle.
  - On disconnect: exponential backoff retry (1s -> 2s -> 4s -> 8s ... max 60s).
  - Parses incoming `ws.Event` JSON and routes `type == "push"` to `notifier`.

## Section 3: Startup & Data Flow

```
Desktop Client Launch
  |
  v
Read config.json
  |
  +-- api_key missing? --+
  |                       |
  v                       v
Connect WS          POST /api/devices/desktop-register
  |                       |
  |                  Save api_key + device_id
  |                       |
  +-----------------------+
  |
  v
Tray shows "Connected"
  |
  v
Backend receives notification
  |
  v
Pipeline.broadcastPushEvent(msg)
  |
  v
Hub sends {"type":"push", payload:{title, body, url}} to all WS clients
  |
  v
Desktop client receives event
  |
  v
Notifier shows native OS notification
  |
  v
User clicks notification -> Browser opens server_url
```

## Section 4: Error Handling

| Scenario | Behavior |
|---|---|
| **WebSocket disconnect** | Exponential backoff retry. Tray icon/menu shows "Disconnected". |
| **Registration failure** (backend unreachable) | Tray shows "Unconfigured". Menu offers "Set Server Address" to update `server_url`. |
| **Notification display failure** | Log error silently. Do not show an error notification to avoid user interruption. |
| **Corrupted or missing config** | Automatically enter first-time registration flow. |
| **Invalid api_key (401 on WS)** | Clear stored `api_key` and re-register. |

## Section 5: Testing

### Backend

- Unit tests for the new migration (schema change).
- Unit tests for `POST /api/devices/desktop-register` handler.
- Unit tests for WebSocket authentication with `api_key`.

### Desktop Client

- Unit tests for `config` read/write.
- Unit tests for `wsclient` event parsing and backoff calculation.
- Manual integration test on **Windows** (the primary target):
  1. Start Overseer backend locally.
  2. Run desktop client. Verify auto-registration and config persistence.
  3. Trigger a test push via `POST /api/push/test`.
  4. Verify native Toast notification appears.
  5. Click notification -> browser opens.
  6. Kill backend, verify tray shows "Disconnected" and retries.
  7. Restart backend, verify auto-reconnect.

## Open Questions / Future Work

- Should the desktop device appear in the Web UI device list with an online/offline status? Not required for MVP.
- Should notifications support the same sound/group/level as Bark? For MVP, only title/body/url are shown; advanced formatting can be added later.
