package escalator

import (
	"fmt"
	"sync"
	"time"

	"github.com/overseer/overseer/internal/config"
	"github.com/overseer/overseer/internal/model"
)

// trackedMessage holds the state for a message being tracked for escalation.
type trackedMessage struct {
	msg             *model.Message
	escalationCount int
	originalSentAt  time.Time
	timer           *time.Timer
}

// Escalator monitors messages that require acknowledgment and escalates them
// through increasingly urgent channels if not confirmed within the configured time.
type Escalator struct {
	config   config.EscalationConfig
	tracked  map[string]*trackedMessage // messageID -> tracked state
	mu       sync.Mutex
	pushFunc func(*model.Message) error
	stopped  bool
}

// NewEscalator creates a new Escalator with the given configuration and push callback.
// The pushFunc is called whenever a message needs to be re-pushed after escalation.
func NewEscalator(cfg config.EscalationConfig, pushFunc func(*model.Message) error) *Escalator {
	return &Escalator{
		config:   cfg,
		tracked:  make(map[string]*trackedMessage),
		pushFunc: pushFunc,
	}
}

// Track begins tracking a message for escalation. If the message is not acknowledged
// within the configured wait time, it will be escalated.
func (e *Escalator) Track(msg *model.Message) {
	if !e.config.Enabled {
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.stopped {
		return
	}

	// If already tracking this message, ignore duplicate track calls
	if _, exists := e.tracked[msg.ID]; exists {
		return
	}

	tm := &trackedMessage{
		msg:             msg,
		escalationCount: 0,
		originalSentAt:  time.Now(),
	}

	waitDuration := e.calculateWait(0)
	tm.timer = time.AfterFunc(waitDuration, func() {
		e.escalate(msg.ID)
	})

	e.tracked[msg.ID] = tm
}

// TrackWithTime is like Track but allows specifying the original sent time.
// This is useful for testing and for restoring state.
func (e *Escalator) TrackWithTime(msg *model.Message, originalSentAt time.Time) {
	if !e.config.Enabled {
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.stopped {
		return
	}

	if _, exists := e.tracked[msg.ID]; exists {
		return
	}

	tm := &trackedMessage{
		msg:             msg,
		escalationCount: 0,
		originalSentAt:  originalSentAt,
	}

	waitDuration := e.calculateWait(0)
	tm.timer = time.AfterFunc(waitDuration, func() {
		e.escalate(msg.ID)
	})

	e.tracked[msg.ID] = tm
}

// Acknowledge cancels all pending escalation plans for the given message ID.
// Returns an error if the message is not being tracked.
func (e *Escalator) Acknowledge(messageID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	tm, exists := e.tracked[messageID]
	if !exists {
		return fmt.Errorf("message %s is not being tracked", messageID)
	}

	tm.timer.Stop()
	delete(e.tracked, messageID)
	return nil
}

// Stop gracefully shuts down the escalator, canceling all pending timers.
func (e *Escalator) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.stopped = true
	for id, tm := range e.tracked {
		tm.timer.Stop()
		delete(e.tracked, id)
	}
}

// TrackedCount returns the number of messages currently being tracked.
func (e *Escalator) TrackedCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.tracked)
}

// escalate is called when a timer fires. It increments the escalation count,
// modifies the message, and re-pushes it. If max escalations reached, it sends
// a final message and stops tracking.
func (e *Escalator) escalate(messageID string) {
	e.mu.Lock()

	tm, exists := e.tracked[messageID]
	if !exists {
		e.mu.Unlock()
		return
	}

	if e.stopped {
		e.mu.Unlock()
		return
	}

	tm.escalationCount++

	// Create a copy of the message for pushing
	escalatedMsg := &model.Message{
		ID:         tm.msg.ID,
		Source:     tm.msg.Source,
		Channel:    tm.msg.Channel,
		Body:       tm.msg.Body,
		Extra:      tm.msg.Extra,
		Status:     tm.msg.Status,
		FailReason: tm.msg.FailReason,
		RetryCount: tm.msg.RetryCount,
		ReceivedAt: tm.msg.ReceivedAt,
		PushedAt:   tm.msg.PushedAt,
	}

	if tm.escalationCount >= e.config.MaxEscalations {
		// Max escalations reached - send final message and stop
		escalatedMsg.Title = fmt.Sprintf("[已达最大升级上限] %s", tm.msg.Title)
		delete(e.tracked, messageID)
		e.mu.Unlock()

		_ = e.pushFunc(escalatedMsg)
		return
	}

	// Normal escalation - annotate with count and original time
	escalatedMsg.Title = fmt.Sprintf("[升级 %d次] 原始时间: %s %s",
		tm.escalationCount,
		tm.originalSentAt.Format("15:04:05"),
		tm.msg.Title,
	)

	// Schedule next escalation
	waitDuration := e.calculateWait(tm.escalationCount)
	tm.timer = time.AfterFunc(waitDuration, func() {
		e.escalate(messageID)
	})

	e.mu.Unlock()

	_ = e.pushFunc(escalatedMsg)
}

// calculateWait returns the wait duration for the given escalation count.
// For "fixed" interval: always uses IntervalMinutes.
// For "increasing" interval: IntervalMinutes * (escalationCount + 1), capped at 120 minutes.
func (e *Escalator) calculateWait(escalationCount int) time.Duration {
	minutes := e.config.IntervalMinutes
	if minutes <= 0 {
		minutes = e.config.WaitMinutes
	}
	if minutes <= 0 {
		minutes = 5 // fallback default
	}

	switch e.config.Interval {
	case "increasing":
		calculated := minutes * (escalationCount + 1)
		if calculated > 120 {
			calculated = 120
		}
		return time.Duration(calculated) * time.Minute
	default: // "fixed" or unspecified
		return time.Duration(minutes) * time.Minute
	}
}
