# Implementation Plan: Notification Lifecycle

## Overview

This plan implements the full notification lifecycle feature in two phases. Phase 1 covers core lifecycle management (unread tracking, acknowledgment, repeat push, snooze, TTL). Phase 2 covers enhanced delivery behaviors (offline catch-up, cross-device sync, message grouping, custom sounds). Tasks are organized by component within each phase: backend first, then desktop client, then web UI.

## Tasks

- [x] 1. Phase 1 — Backend: Data Model & Store Extensions
  - [x] 1.1 Extend Message model with lifecycle fields
    - Add `AckAt`, `SnoozeUntil`, `ExpiresAt`, `RepeatCount` fields to `internal/model/message.go`
    - Add new `PushStatus` constants: `StatusExpiredUnacked`, `StatusExpired`
    - _Requirements: 4.1, 7.2, 9.1, 14.2_

  - [x] 1.2 Extend Channel model with lifecycle configuration fields
    - Add `RequireAck`, `RepeatInterval`, `MaxRepeats`, `SoundFile` fields to `internal/model/channel.go`
    - _Requirements: 6.1, 13.1_

  - [x] 1.3 Add DB migration for lifecycle columns
    - Add `ack_at`, `snooze_until`, `expires_at`, `repeat_count` columns to `messages` table
    - Add `last_seen` column to `devices` table
    - Create indexes `idx_messages_unacked` and `idx_messages_catchup`
    - _Requirements: 4.1, 10.1, 14.2_

  - [x] 1.4 Extend Store interface with lifecycle methods
    - Add `AckMessage`, `SnoozeMessage`, `GetMessage`, `GetUnackedMessages`, `GetCatchUpMessages`, `UpdateRepeatCount`, `ExpireMessage`, `UpdateDeviceLastSeen`, `GetDeviceLastSeen` to `internal/store/store.go`
    - Implement all new methods in the SQLite store implementation
    - _Requirements: 4.1, 7.1, 9.1, 10.1, 10.2_

  - [ ]* 1.5 Write property tests for store lifecycle methods
    - **Property 16: Catch-up returns messages after last_seen**
    - **Property 17: Catch-up messages in chronological order**
    - **Property 18: Catch-up limited to 7 days**
    - **Property 23: TTL computes expires_at correctly**
    - **Validates: Requirements 10.2, 10.3, 10.4, 14.2**

- [x] 2. Phase 1 — Backend: Channel Config Validation
  - [x] 2.1 Add validation for repeat_interval and max_repeats in channel config
    - Validate `repeat_interval` parses to duration between 1m and 24h
    - Validate `max_repeats` is integer between 1 and 100
    - Add validation for `ttl` field (1m to 30 days) in push API
    - Extend `internal/config/validator.go`
    - _Requirements: 6.3, 6.4, 14.5_

  - [ ]* 2.2 Write property tests for channel config validation
    - **Property 8: Channel repeat_interval validation**
    - **Property 9: Channel max_repeats validation**
    - **Property 25: TTL validation**
    - **Validates: Requirements 6.3, 6.4, 14.5**

- [x] 3. Phase 1 — Backend: Ack & Snooze API Endpoints
  - [x] 3.1 Implement LifecycleHandler with HandleAck endpoint
    - Create `internal/server/handler/lifecycle.go`
    - POST `/api/messages/:id/ack`: record `ack_at`, cancel repeat push, return 200/404
    - Handle idempotent ack (already-acked returns 200 unchanged)
    - _Requirements: 4.1, 4.2, 4.3, 4.4_

  - [x] 3.2 Implement HandleSnooze endpoint
    - POST `/api/messages/:id/snooze`: validate duration, record `snooze_until`, schedule re-push
    - Support durations: "1h", "2h", "4h", "tomorrow_9am"
    - Return 200/400/404 as appropriate
    - _Requirements: 9.1, 9.5, 9.6_

  - [x] 3.3 Register lifecycle routes in server router
    - Wire LifecycleHandler into `internal/server/server.go` route registration
    - _Requirements: 4.1, 9.1_

  - [ ]* 3.4 Write property tests for ack/snooze handlers
    - **Property 4: Acknowledgment sets timestamp**
    - **Property 5: Acknowledgment is idempotent**
    - **Property 13: Snooze computes snooze_until correctly**
    - **Validates: Requirements 4.1, 4.4, 9.1**

- [x] 4. Phase 1 — Backend: RepeatPusher Scheduler
  - [x] 4.1 Implement RepeatPusher core scheduler
    - Create `internal/repeatpusher/repeatpusher.go`
    - Implement `Start()` to load eligible unacked messages and schedule timers
    - Implement `Schedule()`, `Cancel()`, `ScheduleSnooze()`, `Stop()`
    - Use `time.AfterFunc` timers backed by DB state (modeled after `internal/scheduler/reminder.go`)
    - _Requirements: 7.1, 7.2, 7.3, 7.5_

  - [x] 4.2 Integrate RepeatPusher with pipeline and server lifecycle
    - Start RepeatPusher on server boot, schedule messages from require_ack channels after pipeline push
    - Cancel repeat push on ack, schedule snooze on snooze endpoint
    - Handle TTL expiry: mark expired messages, stop repeat push
    - _Requirements: 6.2, 7.1, 7.3, 14.3_

  - [ ]* 4.3 Write property tests for RepeatPusher
    - **Property 6: Acknowledged messages excluded from repeat push**
    - **Property 10: Repeat push fires after interval**
    - **Property 11: Repeat push stops at max_repeats**
    - **Property 12: Repeat push resumes after restart**
    - **Property 14: Snoozed messages excluded from repeat push**
    - **Property 15: Snooze expiry triggers re-push**
    - **Property 24: Expired messages excluded from repeat push and unread count**
    - **Validates: Requirements 4.2, 7.1, 7.2, 7.3, 7.5, 9.2, 9.3, 14.3, 14.4**

- [x] 5. Phase 1 — Backend: TTL Processing in Push API
  - [x] 5.1 Accept optional ttl field in push API and compute expires_at
    - Extend push handler in `internal/server/handler/push.go` to parse `ttl` field
    - Compute `expires_at = received_at + parsed(ttl)` and store on message
    - Validate ttl is between 1m and 30 days, return 400 on invalid
    - Messages without ttl have null expires_at (no expiration)
    - _Requirements: 14.1, 14.2, 14.5, 14.6_

- [~] 6. Checkpoint — Phase 1 Backend
  - Ensure all tests pass, ask the user if questions arise.

- [x] 7. Phase 1 — Desktop Client: Unread State & Cache
  - [x] 7.1 Extend local SQLite cache schema with lifecycle columns
    - Add `unread`, `acked_at`, `expires_at` columns to notifications table in `cmd/desktop/internal/cache/cache.go`
    - Implement `MarkRead`, `MarkAllRead`, `UnreadCount`, `MarkAckSynced`, `GetUnexpiredUnread` methods
    - _Requirements: 1.1, 1.4_

  - [ ]* 7.2 Write property tests for cache unread persistence
    - **Property 1: Unread state persistence round-trip**
    - **Validates: Requirements 1.4**

  - [x] 7.3 Implement Unread Tracker component
    - Create `cmd/desktop/internal/unread/tracker.go`
    - Implement `OnPush`, `OnRead`, `OnReadAll`, `OnAckSync`, `OnExpired`, `ShouldFlash`
    - Coordinate between cache state and tray icon controller via callback
    - _Requirements: 1.1, 1.2, 1.3, 2.1, 2.2_

  - [ ]* 7.4 Write property test for tray icon flashing logic
    - **Property 2: Tray icon flashing reflects unread count**
    - **Validates: Requirements 2.1, 2.2, 11.3**

- [x] 8. Phase 1 — Desktop Client: Tray Icon Flashing
  - [x] 8.1 Implement tray icon flashing in tray controller
    - Modify `cmd/desktop/internal/tray/tray.go` to alternate between normal and highlighted icon at 500ms interval when unread > 0
    - Stop flashing and show static icon when all read
    - Start flashing on app launch if existing unread notifications in cache
    - _Requirements: 2.1, 2.2, 2.3_

- [x] 9. Phase 1 — Desktop Client: Toast Action Buttons
  - [x] 9.1 Add "确认" (Ack) and "稍后" (Snooze) buttons to toast notifications
    - Modify `cmd/desktop/internal/notifier/notifier_windows.go` to include action buttons
    - Handle ack button click: POST `/api/messages/:id/ack` with retry
    - Handle snooze button click: POST `/api/messages/:id/snooze` with duration="1h" and retry
    - _Requirements: 3.1, 3.2, 8.1, 8.2_

  - [x] 9.2 Implement exponential backoff retry for ack/snooze POST requests
    - Create retry helper in `cmd/desktop/internal/api/api.go`
    - Retry with delays: 1s, 2s, 4s (base * 2^(i-1)), max 3 attempts
    - _Requirements: 3.3, 8.3_

  - [ ]* 9.3 Write property test for exponential backoff retry
    - **Property 3: Exponential backoff retry**
    - **Validates: Requirements 3.3, 8.3**

- [x] 10. Phase 1 — Desktop Client: Wire Unread Tracker to WS Client
  - [x] 10.1 Integrate unread tracker with WebSocket client event handling
    - Modify `cmd/desktop/internal/wsclient/wsclient.go` to call tracker.OnPush on push events
    - Call tracker.OnRead when toast is interacted with
    - Call tracker.OnReadAll when tray icon is clicked
    - Mark notification as read on ack button click
    - _Requirements: 1.1, 1.2, 1.3_

- [~] 11. Checkpoint — Phase 1 Desktop Client
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 12. Phase 1 — Web UI: Ack Status Display
  - [~] 12.1 Create MessageAckBadge component
    - Create `web/src/components/MessageAckBadge.tsx`
    - Display "已确认 HH:mm" when ack_at is non-null, "未确认" when null
    - _Requirements: 5.1, 5.2_

  - [ ]* 12.2 Write property test for ack status display
    - **Property 7: Ack status display correctness**
    - **Validates: Requirements 5.1, 5.2**

  - [~] 12.3 Integrate MessageAckBadge into message history view
    - Add ack badge to message list items in the history page
    - Handle real-time ack_sync WebSocket events to update badge without page refresh
    - _Requirements: 5.1, 5.2, 5.3_

- [~] 13. Checkpoint — Phase 1 Complete
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 14. Phase 2 — Backend: Offline Catch-Up
  - [~] 14.1 Implement device last_seen tracking via WebSocket ping/pong
    - Update `last_seen` timestamp on each WebSocket ping/pong in `internal/server/handler/ws.go`
    - Call `store.UpdateDeviceLastSeen` on each heartbeat
    - _Requirements: 10.1_

  - [~] 14.2 Implement catch-up handler on WebSocket connection
    - Extend `internal/server/handler/ws.go` with `performCatchUp` function
    - Query unread/unacked messages after device's last_seen (max 7 days)
    - Push catch-up messages in chronological order (oldest first)
    - _Requirements: 10.2, 10.3, 10.4_

- [ ] 15. Phase 2 — Backend: Cross-Device Ack Sync
  - [~] 15.1 Broadcast ack_sync WebSocket event on acknowledgment
    - Extend HandleAck to broadcast `{"type": "ack_sync", "payload": {"message_id": "xxx"}}` to all connected clients except the acknowledging device
    - _Requirements: 11.1_

- [ ] 16. Phase 2 — Desktop Client: Ack Sync Handling
  - [~] 16.1 Handle ack_sync WebSocket events in desktop client
    - Listen for "ack_sync" event type in WS client
    - Call tracker.OnAckSync to mark message as read in local cache
    - Stop tray icon flashing if no remaining unread notifications
    - _Requirements: 11.2, 11.3_

  - [ ]* 16.2 Write property test for ack_sync handling
    - **Property 19: Ack_sync marks message as read**
    - **Validates: Requirements 11.2**

- [ ] 17. Phase 2 — Desktop Client: Message Grouping
  - [~] 17.1 Implement Message Grouper component
    - Create `cmd/desktop/internal/grouper/grouper.go`
    - Batch push events from same source within 30-second window
    - Show single summary toast: "[{source}] {count} new notifications"
    - Use source field as grouping key
    - On grouped toast click, open Web UI filtered to source
    - _Requirements: 12.1, 12.2, 12.3, 12.4_

  - [~] 17.2 Integrate grouper into WS client push event flow
    - Route incoming push events through grouper before notifier
    - Grouper calls notifier for single or grouped display
    - _Requirements: 12.1_

  - [ ]* 17.3 Write property tests for message grouping
    - **Property 20: Message grouping by source within time window**
    - **Property 21: Grouped notification format**
    - **Validates: Requirements 12.1, 12.2, 12.4**

- [ ] 18. Phase 2 — Desktop Client: Custom Notification Sounds
  - [~] 18.1 Implement Sound Player component
    - Create `cmd/desktop/internal/sound/player.go`
    - Download and cache sound files on connect when channel config includes sound_file
    - Play custom sound on notification arrival for configured channels
    - Fall back to level-based default: critical→LoopingAlarm, timeSensitive→Reminder, passive→Silent, default→Default
    - _Requirements: 13.1, 13.2, 13.3, 13.4_

  - [ ]* 18.2 Write property test for sound fallback logic
    - **Property 22: Sound fallback to level-based default**
    - **Validates: Requirements 13.4**

- [ ] 19. Phase 2 — Desktop Client: TTL Expiry Handling
  - [~] 19.1 Handle expired messages in desktop client
    - Check expires_at on push events and cached messages
    - Exclude expired messages from unread count
    - Stop tray icon flashing for expired messages
    - Call tracker.OnExpired when message expires
    - _Requirements: 14.3, 14.4_

- [~] 20. Checkpoint — Phase 2 Complete
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 21. Final Integration & Wiring
  - [~] 21.1 End-to-end integration wiring
    - Verify RepeatPusher integrates with ack/snooze/TTL flows
    - Verify catch-up delivers messages on reconnect
    - Verify ack_sync propagates across devices
    - Verify grouper and sound player work with real push events
    - Write integration tests for key flows: ack→cancel repeat, snooze→re-push, disconnect→catch-up
    - _Requirements: 4.2, 7.3, 9.3, 10.2, 11.1_

- [~] 22. Final Checkpoint
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation between phases and components
- Property tests validate universal correctness properties from the design document
- Unit tests validate specific examples and edge cases
- Phase 1 covers Requirements 1–9, 14 (core lifecycle); Phase 2 covers Requirements 10–13 (enhanced delivery)
- The RepeatPusher generalizes the existing `internal/escalator` package; once stable, escalator can be deprecated

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1", "1.2"] },
    { "id": 1, "tasks": ["1.3", "2.1"] },
    { "id": 2, "tasks": ["1.4", "2.2"] },
    { "id": 3, "tasks": ["1.5", "3.1", "3.2", "5.1"] },
    { "id": 4, "tasks": ["3.3", "3.4"] },
    { "id": 5, "tasks": ["4.1"] },
    { "id": 6, "tasks": ["4.2", "4.3"] },
    { "id": 7, "tasks": ["7.1"] },
    { "id": 8, "tasks": ["7.2", "7.3"] },
    { "id": 9, "tasks": ["7.4", "8.1", "9.1"] },
    { "id": 10, "tasks": ["9.2"] },
    { "id": 11, "tasks": ["9.3", "10.1"] },
    { "id": 12, "tasks": ["12.1"] },
    { "id": 13, "tasks": ["12.2", "12.3"] },
    { "id": 14, "tasks": ["14.1", "15.1"] },
    { "id": 15, "tasks": ["14.2", "16.1"] },
    { "id": 16, "tasks": ["16.2", "17.1"] },
    { "id": 17, "tasks": ["17.2", "17.3", "18.1"] },
    { "id": 18, "tasks": ["18.2", "19.1"] },
    { "id": 19, "tasks": ["21.1"] }
  ]
}
```
