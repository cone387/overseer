# Requirements Document

## Introduction

This feature adds a full notification lifecycle to the Overseer push notification system. Currently, notifications are fire-and-forget — they are pushed once via WebSocket and that is the end of their lifecycle. This feature introduces read/unread state tracking, acknowledgment receipts, persistent reminder mode with repeat pushing, snooze functionality, offline catch-up, cross-device sync, message grouping, custom sounds, and message TTL. The feature is split into two phases: Phase 1 (Core) covers essential lifecycle management, and Phase 2 (Enhanced) covers advanced delivery behaviors.

## Glossary

- **Desktop_Client**: The Go-based system tray application that receives push notifications via WebSocket and displays Windows Toast notifications
- **Backend**: The Go/Gin server that processes, stores, and delivers push notifications
- **Web_UI**: The React/TypeScript frontend for managing notifications, channels, and viewing message history
- **Tray_Icon**: The system tray icon displayed by the Desktop_Client, which can be static or flashing
- **Toast_Notification**: A Windows native notification popup displayed by the Desktop_Client using go-toast/toast
- **WebSocket_Hub**: The server-side component that broadcasts events to all connected WebSocket clients
- **Channel**: A named notification routing target with configurable push parameters (sound, level, device keys)
- **Acknowledgment**: An explicit user action confirming receipt of a notification, recorded as an ack_at timestamp
- **Snooze**: A user action to defer a notification re-push to a later time
- **Repeat_Push**: A backend-driven re-delivery of an unacknowledged message at configured intervals
- **Message_TTL**: A time-to-live duration after which a message expires and stops all lifecycle processing
- **Unread_State**: A locally tracked status on the Desktop_Client indicating whether a notification has been viewed
- **Device_Session**: A WebSocket connection identified by device key, used for tracking online/offline status and last_seen timestamps

## Requirements

### Requirement 1: Unread State Tracking

**User Story:** As a desktop user, I want notifications to have read/unread status tracked locally, so that I can see which notifications I have not yet viewed.

#### Acceptance Criteria

1. WHEN a push event is received via WebSocket, THE Desktop_Client SHALL store the notification in the local cache with an unread status
2. WHEN the user clicks the Tray_Icon, THE Desktop_Client SHALL mark all unread notifications as read
3. WHEN a Toast_Notification is displayed and the user interacts with it, THE Desktop_Client SHALL mark that specific notification as read
4. THE Desktop_Client SHALL persist unread state in the local SQLite cache across application restarts

### Requirement 2: Tray Icon Flashing

**User Story:** As a desktop user, I want the tray icon to flash when there are unread notifications, so that I have a persistent visual indicator of pending messages.

#### Acceptance Criteria

1. WHILE there are one or more unread notifications, THE Desktop_Client SHALL alternate the Tray_Icon between the normal icon and a highlighted icon at a 500ms interval
2. WHEN all notifications are marked as read, THE Desktop_Client SHALL display the static normal Tray_Icon
3. WHEN the Desktop_Client starts with existing unread notifications in the local cache, THE Desktop_Client SHALL begin Tray_Icon flashing immediately

### Requirement 3: Acknowledge Button on Toast Notifications

**User Story:** As a desktop user, I want an "确认" (Acknowledge) button on toast notifications, so that I can explicitly confirm receipt of important messages.

#### Acceptance Criteria

1. WHEN a Toast_Notification is displayed, THE Desktop_Client SHALL include an "确认" (Acknowledge) action button
2. WHEN the user clicks the "确认" button, THE Desktop_Client SHALL send a POST request to /api/messages/:id/ack on the Backend
3. IF the POST /api/messages/:id/ack request fails due to network error, THEN THE Desktop_Client SHALL retry the request with exponential backoff up to 3 attempts

### Requirement 4: Acknowledgment Receipt API

**User Story:** As a system operator, I want the backend to record acknowledgment timestamps, so that I can track which messages have been confirmed by recipients.

#### Acceptance Criteria

1. WHEN a POST /api/messages/:id/ack request is received, THE Backend SHALL record the current timestamp as ack_at on the message record
2. WHEN a message has a non-null ack_at value, THE Backend SHALL stop all Repeat_Push processing for that message
3. IF a POST /api/messages/:id/ack request references a non-existent message ID, THEN THE Backend SHALL return HTTP 404 with an error message
4. IF a POST /api/messages/:id/ack request references an already-acknowledged message, THEN THE Backend SHALL return HTTP 200 without modifying the existing ack_at value

### Requirement 5: Acknowledgment Status in Web UI

**User Story:** As a user viewing message history, I want to see acknowledgment status on each message, so that I know which messages have been confirmed.

#### Acceptance Criteria

1. THE Web_UI SHALL display the ack_at timestamp on messages that have been acknowledged in the message history view
2. THE Web_UI SHALL display an "未确认" (Unacknowledged) indicator on messages that have not been acknowledged
3. WHEN a message is acknowledged while the Web_UI message history is open, THE Web_UI SHALL update the acknowledgment status in real-time via WebSocket event

### Requirement 6: Persistent Reminder Mode Configuration

**User Story:** As a system operator, I want to configure channels for persistent reminder mode, so that critical messages are repeatedly pushed until acknowledged.

#### Acceptance Criteria

1. THE Backend SHALL support the following per-channel configuration fields: require_ack (boolean), repeat_interval (duration string), and max_repeats (integer)
2. WHERE require_ack is set to true on a Channel, THE Backend SHALL enable Repeat_Push processing for all messages delivered on that Channel
3. THE Backend SHALL validate that repeat_interval is a parseable duration between 1 minute and 24 hours
4. THE Backend SHALL validate that max_repeats is an integer between 1 and 100

### Requirement 7: Repeat Push Delivery

**User Story:** As a system operator, I want unacknowledged messages to be re-pushed at intervals, so that critical notifications are not missed.

#### Acceptance Criteria

1. WHEN a message on a require_ack Channel is not acknowledged within the configured repeat_interval, THE Backend SHALL re-push the message via WebSocket to all connected Desktop_Client instances
2. WHEN the number of repeat pushes for a message reaches max_repeats, THE Backend SHALL stop Repeat_Push processing and mark the message status as "expired_unacked"
3. WHEN a message is acknowledged during Repeat_Push processing, THE Backend SHALL cancel all pending repeat pushes for that message
4. THE Backend SHALL include a repeat_count field in the WebSocket push event payload to indicate which repeat delivery this is
5. WHEN the Backend restarts, THE Backend SHALL resume Repeat_Push processing for all unacknowledged messages on require_ack channels that have not reached max_repeats

### Requirement 8: Snooze Button on Toast Notifications

**User Story:** As a desktop user, I want a "稍后" (Snooze) button on toast notifications, so that I can defer a reminder to a more convenient time.

#### Acceptance Criteria

1. WHEN a Toast_Notification is displayed, THE Desktop_Client SHALL include a "稍后" (Snooze) action button
2. WHEN the user clicks the "稍后" button, THE Desktop_Client SHALL send a POST request to /api/messages/:id/snooze with a default snooze duration of 1 hour
3. IF the POST /api/messages/:id/snooze request fails due to network error, THEN THE Desktop_Client SHALL retry the request with exponential backoff up to 3 attempts

### Requirement 9: Snooze Backend Processing

**User Story:** As a system operator, I want the backend to schedule re-pushes at snoozed times, so that deferred notifications are delivered when the user expects them.

#### Acceptance Criteria

1. WHEN a POST /api/messages/:id/snooze request is received with a valid duration, THE Backend SHALL record the snooze_until timestamp on the message
2. WHILE a message is in snoozed state (current time < snooze_until), THE Backend SHALL exclude the message from Repeat_Push processing
3. WHEN the snooze_until time is reached, THE Backend SHALL re-push the message via WebSocket to all connected Desktop_Client instances
4. WHEN a snoozed message is re-pushed after snooze expiry, THE Desktop_Client SHALL treat it as a new unread notification and begin Tray_Icon flashing
5. THE Backend SHALL support the following snooze duration values: "1h", "2h", "4h", "tomorrow_9am"
6. IF a POST /api/messages/:id/snooze request references a non-existent message ID, THEN THE Backend SHALL return HTTP 404

### Requirement 10: Offline Message Catch-Up

**User Story:** As a desktop user, I want to receive all missed notifications when my client reconnects after being offline, so that I do not miss any messages.

#### Acceptance Criteria

1. THE Backend SHALL record a last_seen timestamp per Device_Session, updated each time a WebSocket ping/pong is received
2. WHEN a Desktop_Client establishes a new WebSocket connection, THE Backend SHALL query all unread and unacknowledged messages sent after the device's last_seen timestamp
3. WHEN catch-up messages are identified, THE Backend SHALL push them to the reconnecting Desktop_Client in chronological order (oldest first)
4. THE Backend SHALL limit catch-up replay to messages sent within the last 7 days to prevent excessive replay after long offline periods

### Requirement 11: Cross-Device Acknowledgment Sync

**User Story:** As a user with multiple devices, I want acknowledgments to sync across all my connected desktop clients, so that I do not see stale unread indicators.

#### Acceptance Criteria

1. WHEN a message is acknowledged on any device, THE Backend SHALL broadcast an "ack_sync" WebSocket event containing the message ID to all other connected Desktop_Client instances
2. WHEN a Desktop_Client receives an "ack_sync" event, THE Desktop_Client SHALL mark the referenced message as read in the local cache
3. WHEN a Desktop_Client receives an "ack_sync" event and the referenced message was the only unread notification, THE Desktop_Client SHALL stop Tray_Icon flashing

### Requirement 12: Message Grouping

**User Story:** As a desktop user, I want multiple rapid notifications from the same source to be grouped into a single summary toast, so that I am not overwhelmed by notification spam.

#### Acceptance Criteria

1. WHEN multiple push events from the same source arrive within a 30-second window, THE Desktop_Client SHALL display a single summary Toast_Notification instead of individual notifications
2. THE Desktop_Client SHALL format the grouped Toast_Notification as "[{source}] {count} new notifications"
3. WHEN the user clicks a grouped Toast_Notification, THE Desktop_Client SHALL open the Web_UI filtered to that source
4. THE Desktop_Client SHALL use the source field from the push event payload as the grouping key

### Requirement 13: Custom Notification Sounds Per Channel

**User Story:** As a system operator, I want to configure custom notification sounds per channel, so that users can distinguish notification sources by audio.

#### Acceptance Criteria

1. THE Backend SHALL support an optional sound_file field on Channel configuration pointing to a WAV audio file path
2. WHEN a Desktop_Client connects and channel configuration includes a sound_file, THE Desktop_Client SHALL download and cache the sound file locally
3. WHEN a notification arrives on a Channel with a configured sound_file, THE Desktop_Client SHALL play the custom sound file
4. IF a Channel does not have a sound_file configured, THEN THE Desktop_Client SHALL fall back to the level-based default sound (Default, Reminder, LoopingAlarm, or Silent)

### Requirement 14: Message TTL

**User Story:** As a message sender, I want to specify a time-to-live on messages, so that time-sensitive notifications automatically expire and stop causing alerts.

#### Acceptance Criteria

1. THE Backend SHALL accept an optional ttl field (duration string) in the push API request body
2. WHEN a message with a ttl field is stored, THE Backend SHALL calculate and record an expires_at timestamp (received_at + ttl)
3. WHEN the current time exceeds a message's expires_at timestamp, THE Backend SHALL mark the message as "expired" and stop all Repeat_Push processing
4. WHEN a message is expired, THE Desktop_Client SHALL exclude it from unread count and stop Tray_Icon flashing for that message
5. THE Backend SHALL validate that ttl is a parseable duration between 1 minute and 30 days
6. IF a message has no ttl field, THEN THE Backend SHALL treat the message as having no expiration

