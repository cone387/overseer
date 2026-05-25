package scheduler

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"

	"github.com/overseer/overseer/internal/model"
	"github.com/overseer/overseer/internal/store"
)

// ReminderScheduler manages scheduling and triggering of reminders.
// It uses time.Timer for one-time reminders and robfig/cron for repeating ones.
type ReminderScheduler struct {
	store    store.Store
	timers   map[string]*time.Timer
	cronJobs map[string]cron.EntryID
	cron     *cron.Cron
	pushFunc func(*model.Message) error
	mu       sync.Mutex
	stopCh   chan struct{}
	nowFunc  func() time.Time // injectable for testing
}

// NewReminderScheduler creates a new ReminderScheduler.
// pushFunc is the callback invoked when a reminder triggers.
func NewReminderScheduler(s store.Store, pushFunc func(*model.Message) error) *ReminderScheduler {
	return &ReminderScheduler{
		store:    s,
		timers:   make(map[string]*time.Timer),
		cronJobs: make(map[string]cron.EntryID),
		cron:     cron.New(cron.WithSeconds()),
		pushFunc: pushFunc,
		stopCh:   make(chan struct{}),
		nowFunc:  time.Now,
	}
}

// SetNowFunc sets a custom time function for testing.
func (rs *ReminderScheduler) SetNowFunc(fn func() time.Time) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.nowFunc = fn
}

// Start loads all active reminders from the store and schedules them.
func (rs *ReminderScheduler) Start() error {
	reminders, err := rs.store.GetActiveReminders()
	if err != nil {
		return fmt.Errorf("load active reminders: %w", err)
	}

	rs.cron.Start()

	for i := range reminders {
		if err := rs.Schedule(&reminders[i]); err != nil {
			log.Printf("[reminder-scheduler] failed to schedule reminder %s: %v", reminders[i].ID, err)
		}
	}

	return nil
}

// Schedule sets up a timer or cron job for the given reminder based on its repeat type.
func (rs *ReminderScheduler) Schedule(r *model.Reminder) error {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	// Cancel any existing schedule for this reminder
	rs.cancelLocked(r.ID)

	switch r.RepeatType {
	case model.RepeatOnce:
		return rs.scheduleOnce(r)
	case model.RepeatDaily:
		return rs.scheduleRepeating(r)
	case model.RepeatWeekly:
		return rs.scheduleRepeating(r)
	case model.RepeatCron:
		return rs.scheduleCron(r)
	default:
		return fmt.Errorf("unknown repeat type: %s", r.RepeatType)
	}
}

// scheduleOnce sets a one-shot timer for the reminder's trigger time.
func (rs *ReminderScheduler) scheduleOnce(r *model.Reminder) error {
	triggerAt := r.TriggerAt
	if r.NextTrigger != nil {
		triggerAt = *r.NextTrigger
	}

	now := rs.nowFunc()
	delay := triggerAt.Sub(now)
	if delay <= 0 {
		// Trigger immediately if the time has passed
		delay = 0
	}

	id := r.ID
	timer := time.AfterFunc(delay, func() {
		rs.trigger(id)
	})
	rs.timers[r.ID] = timer
	return nil
}

// scheduleRepeating sets a timer for the next trigger of a daily/weekly reminder.
func (rs *ReminderScheduler) scheduleRepeating(r *model.Reminder) error {
	triggerAt := r.TriggerAt
	if r.NextTrigger != nil {
		triggerAt = *r.NextTrigger
	}

	now := rs.nowFunc()
	delay := triggerAt.Sub(now)
	if delay <= 0 {
		// If the next trigger is in the past, calculate the correct next time
		triggerAt = rs.computeNextTrigger(r, now)
		delay = triggerAt.Sub(now)
		if delay <= 0 {
			delay = 0
		}
		// Update the store with the new next trigger
		if err := rs.store.UpdateNextTrigger(r.ID, triggerAt); err != nil {
			log.Printf("[reminder-scheduler] failed to update next trigger for %s: %v", r.ID, err)
		}
	}

	id := r.ID
	timer := time.AfterFunc(delay, func() {
		rs.trigger(id)
	})
	rs.timers[r.ID] = timer
	return nil
}

// scheduleCron sets up a cron job for the reminder using its cron expression.
func (rs *ReminderScheduler) scheduleCron(r *model.Reminder) error {
	if r.RepeatRule == "" {
		return fmt.Errorf("cron reminder %s has empty repeat_rule", r.ID)
	}

	id := r.ID
	entryID, err := rs.cron.AddFunc(r.RepeatRule, func() {
		rs.trigger(id)
	})
	if err != nil {
		return fmt.Errorf("add cron job for reminder %s: %w", r.ID, err)
	}
	rs.cronJobs[r.ID] = entryID
	return nil
}

// trigger is called when a reminder fires. It constructs a message and pushes it.
func (rs *ReminderScheduler) trigger(reminderID string) {
	rs.mu.Lock()
	// Remove the timer reference since it has fired
	delete(rs.timers, reminderID)
	rs.mu.Unlock()

	// Load the reminder from store to get current state
	reminders, err := rs.store.ListReminders(store.ReminderFilter{Status: "active"})
	if err != nil {
		log.Printf("[reminder-scheduler] failed to load reminder %s: %v", reminderID, err)
		return
	}

	var reminder *model.Reminder
	for i := range reminders {
		if reminders[i].ID == reminderID {
			reminder = &reminders[i]
			break
		}
	}
	if reminder == nil {
		// Reminder was cancelled or deleted
		return
	}

	// Construct the push message
	msg := &model.Message{
		ID:         uuid.New().String(),
		Source:     "reminder",
		Channel:    reminder.Channel,
		Title:      reminder.Title,
		Body:       reminder.Body,
		Status:     model.StatusPending,
		ReceivedAt: rs.nowFunc(),
	}

	// Attempt to push
	if err := rs.pushFunc(msg); err != nil {
		log.Printf("[reminder-scheduler] push failed for reminder %s, retrying in 1 minute: %v", reminderID, err)
		// Retry once after 1 minute
		rs.retryAfter(reminderID, reminder, msg, 1*time.Minute)
		return
	}

	// Push succeeded - update reminder state
	rs.afterTriggerSuccess(reminder)
}

// retryAfter schedules a single retry after the given delay.
func (rs *ReminderScheduler) retryAfter(reminderID string, reminder *model.Reminder, msg *model.Message, delay time.Duration) {
	rs.mu.Lock()
	timer := time.AfterFunc(delay, func() {
		rs.mu.Lock()
		delete(rs.timers, reminderID+"_retry")
		rs.mu.Unlock()

		if err := rs.pushFunc(msg); err != nil {
			log.Printf("[reminder-scheduler] retry push failed for reminder %s, marking as failed: %v", reminderID, err)
			// Mark reminder as trigger failed
			rs.markFailed(reminder, err.Error())
			return
		}
		// Retry succeeded
		rs.afterTriggerSuccess(reminder)
	})
	rs.timers[reminderID+"_retry"] = timer
	rs.mu.Unlock()
}

// afterTriggerSuccess handles post-trigger logic: update last_triggered, compute next trigger or mark completed.
func (rs *ReminderScheduler) afterTriggerSuccess(reminder *model.Reminder) {
	now := rs.nowFunc()
	reminder.LastTriggered = &now

	switch reminder.RepeatType {
	case model.RepeatOnce:
		// Mark as completed
		reminder.Status = "completed"
		reminder.UpdatedAt = now
		if err := rs.store.UpdateReminder(reminder); err != nil {
			log.Printf("[reminder-scheduler] failed to mark reminder %s as completed: %v", reminder.ID, err)
		}
	case model.RepeatDaily, model.RepeatWeekly:
		// Calculate next trigger and reschedule
		next := rs.computeNextTrigger(reminder, now)
		reminder.NextTrigger = &next
		reminder.UpdatedAt = now
		if err := rs.store.UpdateNextTrigger(reminder.ID, next); err != nil {
			log.Printf("[reminder-scheduler] failed to update next trigger for %s: %v", reminder.ID, err)
		}
		// Reschedule
		rs.mu.Lock()
		delay := next.Sub(rs.nowFunc())
		if delay <= 0 {
			delay = 0
		}
		id := reminder.ID
		timer := time.AfterFunc(delay, func() {
			rs.trigger(id)
		})
		rs.timers[reminder.ID] = timer
		rs.mu.Unlock()
	case model.RepeatCron:
		// Cron jobs are managed by the cron library, just update last_triggered
		reminder.UpdatedAt = now
		if err := rs.store.UpdateNextTrigger(reminder.ID, rs.nextCronTrigger(reminder)); err != nil {
			log.Printf("[reminder-scheduler] failed to update next trigger for cron reminder %s: %v", reminder.ID, err)
		}
	}
}

// computeNextTrigger calculates the next trigger time based on repeat type.
func (rs *ReminderScheduler) computeNextTrigger(r *model.Reminder, from time.Time) time.Time {
	switch r.RepeatType {
	case model.RepeatDaily:
		return from.Add(24 * time.Hour)
	case model.RepeatWeekly:
		return from.Add(7 * 24 * time.Hour)
	default:
		return from
	}
}

// nextCronTrigger returns the next trigger time for a cron reminder.
func (rs *ReminderScheduler) nextCronTrigger(r *model.Reminder) time.Time {
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(r.RepeatRule)
	if err != nil {
		return rs.nowFunc().Add(time.Hour) // fallback
	}
	return schedule.Next(rs.nowFunc())
}

// markFailed marks a reminder as trigger-failed in the store.
func (rs *ReminderScheduler) markFailed(reminder *model.Reminder, reason string) {
	now := rs.nowFunc()
	reminder.Status = "failed"
	reminder.FailReason = reason
	reminder.LastTriggered = &now
	reminder.UpdatedAt = now
	if err := rs.store.UpdateReminder(reminder); err != nil {
		log.Printf("[reminder-scheduler] failed to mark reminder %s as failed: %v", reminder.ID, err)
	}
}

// Cancel cancels the scheduled timer or cron job for the given reminder ID.
func (rs *ReminderScheduler) Cancel(id string) error {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.cancelLocked(id)
	return nil
}

// cancelLocked cancels a reminder's timer/cron without acquiring the lock.
// Caller must hold rs.mu.
func (rs *ReminderScheduler) cancelLocked(id string) {
	if timer, ok := rs.timers[id]; ok {
		timer.Stop()
		delete(rs.timers, id)
	}
	// Also cancel any pending retry timer
	if timer, ok := rs.timers[id+"_retry"]; ok {
		timer.Stop()
		delete(rs.timers, id+"_retry")
	}
	if entryID, ok := rs.cronJobs[id]; ok {
		rs.cron.Remove(entryID)
		delete(rs.cronJobs, id)
	}
}

// Stop gracefully shuts down the scheduler, cancelling all timers and stopping cron.
func (rs *ReminderScheduler) Stop() {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	// Stop all timers
	for id, timer := range rs.timers {
		timer.Stop()
		delete(rs.timers, id)
	}

	// Stop cron (waits for running jobs to complete)
	ctx := rs.cron.Stop()
	<-ctx.Done()

	// Clear cron jobs map
	for id := range rs.cronJobs {
		delete(rs.cronJobs, id)
	}

	close(rs.stopCh)
}
