package repeatpusher

import (
	"log"
	"sync"
	"time"

	"github.com/overseer/overseer/internal/config"
	"github.com/overseer/overseer/internal/model"
	"github.com/overseer/overseer/internal/store"
	"github.com/overseer/overseer/internal/ws"
)

// RepeatPusher manages periodic re-delivery of unacknowledged messages
// on channels with require_ack: true. It is modeled after the ReminderScheduler
// and uses time.AfterFunc timers backed by DB state.
type RepeatPusher struct {
	store    store.Store
	hub      *ws.Hub
	channels []config.Channel  // channels with require_ack=true
	timers   map[string]*time.Timer // messageID -> timer
	mu       sync.Mutex
	stopCh   chan struct{}
	nowFunc  func() time.Time // injectable for testing
}

// New creates a new RepeatPusher for the given require_ack channels.
func New(store store.Store, hub *ws.Hub, channels []config.Channel) *RepeatPusher {
	return &RepeatPusher{
		store:    store,
		hub:      hub,
		channels: channels,
		timers:   make(map[string]*time.Timer),
		stopCh:   make(chan struct{}),
		nowFunc:  time.Now,
	}
}

// SetNowFunc sets a custom time function for testing.
func (rp *RepeatPusher) SetNowFunc(fn func() time.Time) {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	rp.nowFunc = fn
}

// Start loads all eligible unacked messages from DB and schedules timers.
func (rp *RepeatPusher) Start() error {
	channelNames := make([]string, len(rp.channels))
	for i, ch := range rp.channels {
		channelNames[i] = ch.Name
	}

	if len(channelNames) == 0 {
		return nil
	}

	messages, err := rp.store.GetUnackedMessages(channelNames)
	if err != nil {
		return err
	}

	for i := range messages {
		msg := &messages[i]
		ch := rp.findChannel(msg.Channel)
		if ch == nil {
			continue
		}

		// Skip messages that have reached max_repeats
		if msg.RepeatCount >= ch.MaxRepeats {
			continue
		}

		// Skip expired messages
		if msg.ExpiresAt != nil && rp.nowFunc().After(*msg.ExpiresAt) {
			continue
		}

		// Skip snoozed messages — they get scheduled via ScheduleSnooze
		if msg.SnoozeUntil != nil && rp.nowFunc().Before(*msg.SnoozeUntil) {
			rp.scheduleSnoozeTimer(msg.ID, *msg.SnoozeUntil)
			continue
		}

		rp.scheduleTimer(msg)
	}

	return nil
}

// Schedule begins tracking a message for repeat push.
// Called after a message is pushed on a require_ack channel.
func (rp *RepeatPusher) Schedule(msg *model.Message) error {
	ch := rp.findChannel(msg.Channel)
	if ch == nil {
		return nil
	}

	rp.scheduleTimer(msg)
	return nil
}

// Cancel stops repeat push for a message (called on ack).
func (rp *RepeatPusher) Cancel(messageID string) {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	if timer, ok := rp.timers[messageID]; ok {
		timer.Stop()
		delete(rp.timers, messageID)
	}
}

// ScheduleSnooze defers a message's next push to snooze_until.
func (rp *RepeatPusher) ScheduleSnooze(messageID string, until time.Time) {
	// Cancel existing timer first
	rp.Cancel(messageID)

	rp.scheduleSnoozeTimer(messageID, until)
}

// Stop gracefully shuts down all timers.
func (rp *RepeatPusher) Stop() {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	for id, timer := range rp.timers {
		timer.Stop()
		delete(rp.timers, id)
	}

	close(rp.stopCh)
}

// scheduleTimer sets a timer for the next repeat push of a message.
func (rp *RepeatPusher) scheduleTimer(msg *model.Message) {
	ch := rp.findChannel(msg.Channel)
	if ch == nil {
		return
	}

	interval, err := time.ParseDuration(ch.RepeatInterval)
	if err != nil {
		log.Printf("[repeat-pusher] invalid repeat_interval for channel %s: %v", ch.Name, err)
		return
	}

	rp.mu.Lock()
	defer rp.mu.Unlock()

	// Cancel any existing timer for this message
	if timer, ok := rp.timers[msg.ID]; ok {
		timer.Stop()
		delete(rp.timers, msg.ID)
	}

	delay := interval

	msgID := msg.ID
	timer := time.AfterFunc(delay, func() {
		rp.onTimerFired(msgID)
	})
	rp.timers[msg.ID] = timer
}

// scheduleSnoozeTimer sets a timer to fire at the snooze_until time.
func (rp *RepeatPusher) scheduleSnoozeTimer(messageID string, until time.Time) {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	// Cancel any existing timer
	if timer, ok := rp.timers[messageID]; ok {
		timer.Stop()
		delete(rp.timers, messageID)
	}

	now := rp.nowFunc()
	delay := until.Sub(now)
	if delay <= 0 {
		delay = 0
	}

	timer := time.AfterFunc(delay, func() {
		rp.onTimerFired(messageID)
	})
	rp.timers[messageID] = timer
}

// onTimerFired is called when a repeat push timer fires.
func (rp *RepeatPusher) onTimerFired(messageID string) {
	rp.mu.Lock()
	delete(rp.timers, messageID)
	rp.mu.Unlock()

	// Load current message state from DB
	msg, err := rp.store.GetMessage(messageID)
	if err != nil {
		log.Printf("[repeat-pusher] failed to load message %s: %v", messageID, err)
		return
	}
	if msg == nil {
		return
	}

	// Check if message has been acknowledged
	if msg.AckAt != nil {
		return
	}

	// Check if message is snoozed (snooze_until in the future)
	now := rp.nowFunc()
	if msg.SnoozeUntil != nil && now.Before(*msg.SnoozeUntil) {
		// Re-schedule for snooze_until
		rp.scheduleSnoozeTimer(messageID, *msg.SnoozeUntil)
		return
	}

	// Check if message is expired
	if msg.ExpiresAt != nil && now.After(*msg.ExpiresAt) {
		// Mark as expired, don't re-push
		if err := rp.store.ExpireMessage(messageID); err != nil {
			log.Printf("[repeat-pusher] failed to expire message %s: %v", messageID, err)
		}
		return
	}

	// Find channel config
	ch := rp.findChannel(msg.Channel)
	if ch == nil {
		return
	}

	// Increment repeat count
	msg.RepeatCount++

	// Check if max_repeats reached
	if msg.RepeatCount >= ch.MaxRepeats {
		// Mark as expired_unacked and stop
		if err := rp.store.UpdateMessageStatus(messageID, model.StatusExpiredUnacked, ""); err != nil {
			log.Printf("[repeat-pusher] failed to mark message %s as expired_unacked: %v", messageID, err)
		}
		return
	}

	// Update repeat count in DB
	if err := rp.store.UpdateRepeatCount(messageID, msg.RepeatCount); err != nil {
		log.Printf("[repeat-pusher] failed to update repeat_count for message %s: %v", messageID, err)
	}

	// Broadcast push event via hub
	rp.hub.Broadcast(ws.Event{
		Type: "push",
		Payload: map[string]interface{}{
			"id":           msg.ID,
			"title":        msg.Title,
			"body":         msg.Body,
			"channel":      msg.Channel,
			"source":       msg.Source,
			"repeat_count": msg.RepeatCount,
			"url":          msg.URL,
			"icon":         msg.Icon,
			"level":        msg.Level,
			"group":        msg.Group,
			"time":         msg.ReceivedAt.Format(time.RFC3339),
		},
	})

	// Reschedule for next interval
	rp.scheduleTimer(msg)
}

// findChannel returns the channel config for the given channel name, or nil if not found.
func (rp *RepeatPusher) findChannel(name string) *config.Channel {
	for i := range rp.channels {
		if rp.channels[i].Name == name {
			return &rp.channels[i]
		}
	}
	return nil
}

// TrackedCount returns the number of messages currently being tracked.
// Useful for testing and monitoring.
func (rp *RepeatPusher) TrackedCount() int {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	return len(rp.timers)
}
