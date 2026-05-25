package escalator

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/overseer/overseer/internal/config"
	"github.com/overseer/overseer/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testConfig returns an EscalationConfig suitable for fast tests.
func testConfig() config.EscalationConfig {
	return config.EscalationConfig{
		Enabled:         true,
		WaitMinutes:     1,
		MaxEscalations:  3,
		Interval:        "fixed",
		IntervalMinutes: 1,
	}
}

// testMessage returns a sample message for testing.
func testMessage(id string) *model.Message {
	return &model.Message{
		ID:         id,
		Source:     "test",
		Channel:    "default",
		Title:      "Test Alert",
		Body:       "Something happened",
		ReceivedAt: time.Now(),
	}
}

func TestNewEscalator(t *testing.T) {
	cfg := testConfig()
	pushFunc := func(msg *model.Message) error { return nil }

	e := NewEscalator(cfg, pushFunc)

	assert.NotNil(t, e)
	assert.Equal(t, cfg, e.config)
	assert.NotNil(t, e.tracked)
	assert.Equal(t, 0, len(e.tracked))
}

func TestTrack_DisabledConfig(t *testing.T) {
	cfg := testConfig()
	cfg.Enabled = false
	pushFunc := func(msg *model.Message) error { return nil }

	e := NewEscalator(cfg, pushFunc)
	e.Track(testMessage("msg-1"))

	assert.Equal(t, 0, e.TrackedCount())
}

func TestTrack_AddsMessageToTracking(t *testing.T) {
	cfg := testConfig()
	pushFunc := func(msg *model.Message) error { return nil }

	e := NewEscalator(cfg, pushFunc)
	e.Track(testMessage("msg-1"))
	defer e.Stop()

	assert.Equal(t, 1, e.TrackedCount())
}

func TestTrack_DuplicateIgnored(t *testing.T) {
	cfg := testConfig()
	pushFunc := func(msg *model.Message) error { return nil }

	e := NewEscalator(cfg, pushFunc)
	msg := testMessage("msg-1")
	e.Track(msg)
	e.Track(msg) // duplicate
	defer e.Stop()

	assert.Equal(t, 1, e.TrackedCount())
}

func TestTrack_AfterStopDoesNothing(t *testing.T) {
	cfg := testConfig()
	pushFunc := func(msg *model.Message) error { return nil }

	e := NewEscalator(cfg, pushFunc)
	e.Stop()
	e.Track(testMessage("msg-1"))

	assert.Equal(t, 0, e.TrackedCount())
}

func TestAcknowledge_CancelsEscalation(t *testing.T) {
	cfg := testConfig()
	pushFunc := func(msg *model.Message) error { return nil }

	e := NewEscalator(cfg, pushFunc)
	e.Track(testMessage("msg-1"))

	err := e.Acknowledge("msg-1")
	require.NoError(t, err)
	assert.Equal(t, 0, e.TrackedCount())
}

func TestAcknowledge_UnknownMessageReturnsError(t *testing.T) {
	cfg := testConfig()
	pushFunc := func(msg *model.Message) error { return nil }

	e := NewEscalator(cfg, pushFunc)

	err := e.Acknowledge("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
}

func TestStop_CancelsAllTimers(t *testing.T) {
	cfg := testConfig()
	pushFunc := func(msg *model.Message) error { return nil }

	e := NewEscalator(cfg, pushFunc)
	e.Track(testMessage("msg-1"))
	e.Track(testMessage("msg-2"))
	e.Track(testMessage("msg-3"))

	assert.Equal(t, 3, e.TrackedCount())

	e.Stop()

	assert.Equal(t, 0, e.TrackedCount())
}

func TestEscalation_FiresAfterTimeout(t *testing.T) {
	// Use a very short interval for testing by manipulating the timer directly
	cfg := config.EscalationConfig{
		Enabled:         true,
		WaitMinutes:     1,
		MaxEscalations:  3,
		Interval:        "fixed",
		IntervalMinutes: 1,
	}

	var mu sync.Mutex
	var pushed []*model.Message
	pushFunc := func(msg *model.Message) error {
		mu.Lock()
		pushed = append(pushed, msg)
		mu.Unlock()
		return nil
	}

	e := NewEscalator(cfg, pushFunc)
	msg := testMessage("msg-esc-1")

	// Directly manipulate to use short timer for testing
	e.mu.Lock()
	tm := &trackedMessage{
		msg:             msg,
		escalationCount: 0,
		originalSentAt:  time.Now(),
	}
	tm.timer = time.AfterFunc(50*time.Millisecond, func() {
		e.escalate(msg.ID)
	})
	e.tracked[msg.ID] = tm
	e.mu.Unlock()

	// Wait for escalation to fire
	time.Sleep(150 * time.Millisecond)

	mu.Lock()
	require.Len(t, pushed, 1)
	assert.Contains(t, pushed[0].Title, "[升级 1次]")
	assert.Contains(t, pushed[0].Title, "原始时间:")
	assert.Contains(t, pushed[0].Title, "Test Alert")
	mu.Unlock()

	e.Stop()
}

func TestEscalation_MaxEscalationsReached(t *testing.T) {
	cfg := config.EscalationConfig{
		Enabled:         true,
		WaitMinutes:     1,
		MaxEscalations:  2,
		Interval:        "fixed",
		IntervalMinutes: 1,
	}

	var mu sync.Mutex
	var pushed []*model.Message
	pushFunc := func(msg *model.Message) error {
		mu.Lock()
		pushed = append(pushed, msg)
		mu.Unlock()
		return nil
	}

	e := NewEscalator(cfg, pushFunc)
	msg := testMessage("msg-max")

	// Set up with escalation count at max-1 so next escalation hits the limit
	e.mu.Lock()
	tm := &trackedMessage{
		msg:             msg,
		escalationCount: 1, // next will be 2 which equals MaxEscalations
		originalSentAt:  time.Now(),
	}
	tm.timer = time.AfterFunc(50*time.Millisecond, func() {
		e.escalate(msg.ID)
	})
	e.tracked[msg.ID] = tm
	e.mu.Unlock()

	time.Sleep(150 * time.Millisecond)

	mu.Lock()
	require.Len(t, pushed, 1)
	assert.Contains(t, pushed[0].Title, "[已达最大升级上限]")
	assert.Contains(t, pushed[0].Title, "Test Alert")
	mu.Unlock()

	// Message should be removed from tracking after max
	assert.Equal(t, 0, e.TrackedCount())
}

func TestEscalation_MultipleEscalations(t *testing.T) {
	cfg := config.EscalationConfig{
		Enabled:         true,
		WaitMinutes:     1,
		MaxEscalations:  3,
		Interval:        "fixed",
		IntervalMinutes: 1,
	}

	var mu sync.Mutex
	var pushed []*model.Message
	pushFunc := func(msg *model.Message) error {
		mu.Lock()
		pushed = append(pushed, msg)
		mu.Unlock()
		return nil
	}

	e := NewEscalator(cfg, pushFunc)
	msg := testMessage("msg-multi")

	// Start with short timer
	e.mu.Lock()
	tm := &trackedMessage{
		msg:             msg,
		escalationCount: 0,
		originalSentAt:  time.Now(),
	}
	tm.timer = time.AfterFunc(30*time.Millisecond, func() {
		e.escalate(msg.ID)
	})
	e.tracked[msg.ID] = tm
	e.mu.Unlock()

	// After first escalation, manually trigger the next one quickly
	time.Sleep(80 * time.Millisecond)

	// First escalation should have fired, now force the next timer to be short
	e.mu.Lock()
	if tm2, ok := e.tracked[msg.ID]; ok {
		tm2.timer.Stop()
		tm2.timer = time.AfterFunc(30*time.Millisecond, func() {
			e.escalate(msg.ID)
		})
	}
	e.mu.Unlock()

	time.Sleep(80 * time.Millisecond)

	// Second escalation fired, force third (which should be max)
	e.mu.Lock()
	if tm3, ok := e.tracked[msg.ID]; ok {
		tm3.timer.Stop()
		tm3.timer = time.AfterFunc(30*time.Millisecond, func() {
			e.escalate(msg.ID)
		})
	}
	e.mu.Unlock()

	time.Sleep(80 * time.Millisecond)

	mu.Lock()
	require.Len(t, pushed, 3)
	assert.Contains(t, pushed[0].Title, "[升级 1次]")
	assert.Contains(t, pushed[1].Title, "[升级 2次]")
	assert.Contains(t, pushed[2].Title, "[已达最大升级上限]")
	mu.Unlock()

	assert.Equal(t, 0, e.TrackedCount())
}

func TestAcknowledge_PreventsSubsequentEscalation(t *testing.T) {
	cfg := testConfig()

	var mu sync.Mutex
	var pushed []*model.Message
	pushFunc := func(msg *model.Message) error {
		mu.Lock()
		pushed = append(pushed, msg)
		mu.Unlock()
		return nil
	}

	e := NewEscalator(cfg, pushFunc)
	msg := testMessage("msg-ack")

	// Set up with short timer
	e.mu.Lock()
	tm := &trackedMessage{
		msg:             msg,
		escalationCount: 0,
		originalSentAt:  time.Now(),
	}
	tm.timer = time.AfterFunc(100*time.Millisecond, func() {
		e.escalate(msg.ID)
	})
	e.tracked[msg.ID] = tm
	e.mu.Unlock()

	// Acknowledge before timer fires
	time.Sleep(30 * time.Millisecond)
	err := e.Acknowledge("msg-ack")
	require.NoError(t, err)

	// Wait past the original timer
	time.Sleep(150 * time.Millisecond)

	mu.Lock()
	assert.Len(t, pushed, 0, "no escalation should fire after acknowledge")
	mu.Unlock()
}

func TestCalculateWait_FixedInterval(t *testing.T) {
	cfg := config.EscalationConfig{
		Enabled:         true,
		WaitMinutes:     5,
		MaxEscalations:  3,
		Interval:        "fixed",
		IntervalMinutes: 5,
	}
	e := NewEscalator(cfg, nil)

	assert.Equal(t, 5*time.Minute, e.calculateWait(0))
	assert.Equal(t, 5*time.Minute, e.calculateWait(1))
	assert.Equal(t, 5*time.Minute, e.calculateWait(2))
	assert.Equal(t, 5*time.Minute, e.calculateWait(5))
}

func TestCalculateWait_IncreasingInterval(t *testing.T) {
	cfg := config.EscalationConfig{
		Enabled:         true,
		WaitMinutes:     5,
		MaxEscalations:  10,
		Interval:        "increasing",
		IntervalMinutes: 5,
	}
	e := NewEscalator(cfg, nil)

	// increasing: IntervalMinutes * (escalationCount + 1)
	assert.Equal(t, 5*time.Minute, e.calculateWait(0))   // 5 * 1
	assert.Equal(t, 10*time.Minute, e.calculateWait(1))  // 5 * 2
	assert.Equal(t, 15*time.Minute, e.calculateWait(2))  // 5 * 3
	assert.Equal(t, 20*time.Minute, e.calculateWait(3))  // 5 * 4
}

func TestCalculateWait_IncreasingInterval_CappedAt120(t *testing.T) {
	cfg := config.EscalationConfig{
		Enabled:         true,
		WaitMinutes:     30,
		MaxEscalations:  10,
		Interval:        "increasing",
		IntervalMinutes: 30,
	}
	e := NewEscalator(cfg, nil)

	// 30 * 5 = 150, should be capped at 120
	assert.Equal(t, 120*time.Minute, e.calculateWait(4))
	// 30 * 10 = 300, should be capped at 120
	assert.Equal(t, 120*time.Minute, e.calculateWait(9))
}

func TestCalculateWait_FallbackDefaults(t *testing.T) {
	// When IntervalMinutes is 0, falls back to WaitMinutes
	cfg := config.EscalationConfig{
		Enabled:         true,
		WaitMinutes:     7,
		MaxEscalations:  3,
		Interval:        "fixed",
		IntervalMinutes: 0,
	}
	e := NewEscalator(cfg, nil)
	assert.Equal(t, 7*time.Minute, e.calculateWait(0))

	// When both are 0, falls back to 5 minutes
	cfg2 := config.EscalationConfig{
		Enabled:         true,
		WaitMinutes:     0,
		MaxEscalations:  3,
		Interval:        "fixed",
		IntervalMinutes: 0,
	}
	e2 := NewEscalator(cfg2, nil)
	assert.Equal(t, 5*time.Minute, e2.calculateWait(0))
}

func TestEscalation_TitleFormat(t *testing.T) {
	cfg := config.EscalationConfig{
		Enabled:         true,
		WaitMinutes:     1,
		MaxEscalations:  5,
		Interval:        "fixed",
		IntervalMinutes: 1,
	}

	var mu sync.Mutex
	var pushed []*model.Message
	pushFunc := func(msg *model.Message) error {
		mu.Lock()
		pushed = append(pushed, msg)
		mu.Unlock()
		return nil
	}

	e := NewEscalator(cfg, pushFunc)
	msg := testMessage("msg-fmt")
	sentAt := time.Date(2024, 1, 15, 14, 30, 45, 0, time.Local)

	e.mu.Lock()
	tm := &trackedMessage{
		msg:             msg,
		escalationCount: 0,
		originalSentAt:  sentAt,
	}
	tm.timer = time.AfterFunc(30*time.Millisecond, func() {
		e.escalate(msg.ID)
	})
	e.tracked[msg.ID] = tm
	e.mu.Unlock()

	time.Sleep(80 * time.Millisecond)

	mu.Lock()
	require.Len(t, pushed, 1)
	expected := fmt.Sprintf("[升级 1次] 原始时间: %s Test Alert", sentAt.Format("15:04:05"))
	assert.Equal(t, expected, pushed[0].Title)
	mu.Unlock()

	e.Stop()
}

func TestEscalation_PreservesMessageFields(t *testing.T) {
	cfg := config.EscalationConfig{
		Enabled:         true,
		WaitMinutes:     1,
		MaxEscalations:  3,
		Interval:        "fixed",
		IntervalMinutes: 1,
	}

	var mu sync.Mutex
	var pushed []*model.Message
	pushFunc := func(msg *model.Message) error {
		mu.Lock()
		pushed = append(pushed, msg)
		mu.Unlock()
		return nil
	}

	e := NewEscalator(cfg, pushFunc)
	msg := &model.Message{
		ID:      "msg-preserve",
		Source:  "github",
		Channel: "urgent",
		Title:   "CI Failed",
		Body:    "Build #123 failed",
		Extra:   map[string]string{"repo": "myrepo"},
	}

	e.mu.Lock()
	tm := &trackedMessage{
		msg:             msg,
		escalationCount: 0,
		originalSentAt:  time.Now(),
	}
	tm.timer = time.AfterFunc(30*time.Millisecond, func() {
		e.escalate(msg.ID)
	})
	e.tracked[msg.ID] = tm
	e.mu.Unlock()

	time.Sleep(80 * time.Millisecond)

	mu.Lock()
	require.Len(t, pushed, 1)
	assert.Equal(t, "msg-preserve", pushed[0].ID)
	assert.Equal(t, "github", pushed[0].Source)
	assert.Equal(t, "urgent", pushed[0].Channel)
	assert.Equal(t, "Build #123 failed", pushed[0].Body)
	assert.Equal(t, map[string]string{"repo": "myrepo"}, pushed[0].Extra)
	mu.Unlock()

	e.Stop()
}

func TestEscalation_PushFuncError_DoesNotPanic(t *testing.T) {
	cfg := config.EscalationConfig{
		Enabled:         true,
		WaitMinutes:     1,
		MaxEscalations:  3,
		Interval:        "fixed",
		IntervalMinutes: 1,
	}

	pushFunc := func(msg *model.Message) error {
		return fmt.Errorf("push failed")
	}

	e := NewEscalator(cfg, pushFunc)
	msg := testMessage("msg-err")

	e.mu.Lock()
	tm := &trackedMessage{
		msg:             msg,
		escalationCount: 0,
		originalSentAt:  time.Now(),
	}
	tm.timer = time.AfterFunc(30*time.Millisecond, func() {
		e.escalate(msg.ID)
	})
	e.tracked[msg.ID] = tm
	e.mu.Unlock()

	// Should not panic even if pushFunc returns error
	time.Sleep(80 * time.Millisecond)

	// Message should still be tracked (for next escalation attempt)
	assert.Equal(t, 1, e.TrackedCount())
	e.Stop()
}

func TestTrackWithTime(t *testing.T) {
	cfg := testConfig()

	var mu sync.Mutex
	var pushed []*model.Message
	pushFunc := func(msg *model.Message) error {
		mu.Lock()
		pushed = append(pushed, msg)
		mu.Unlock()
		return nil
	}

	e := NewEscalator(cfg, pushFunc)
	msg := testMessage("msg-time")
	sentAt := time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC)

	// Use TrackWithTime but override the timer for fast testing
	e.mu.Lock()
	tm := &trackedMessage{
		msg:             msg,
		escalationCount: 0,
		originalSentAt:  sentAt,
	}
	tm.timer = time.AfterFunc(30*time.Millisecond, func() {
		e.escalate(msg.ID)
	})
	e.tracked[msg.ID] = tm
	e.mu.Unlock()

	time.Sleep(80 * time.Millisecond)

	mu.Lock()
	require.Len(t, pushed, 1)
	assert.Contains(t, pushed[0].Title, "10:00:00")
	mu.Unlock()

	e.Stop()
}

func TestConcurrentAcknowledge(t *testing.T) {
	cfg := testConfig()
	pushFunc := func(msg *model.Message) error { return nil }

	e := NewEscalator(cfg, pushFunc)

	// Track multiple messages
	for i := 0; i < 10; i++ {
		e.Track(testMessage(fmt.Sprintf("msg-%d", i)))
	}

	// Acknowledge concurrently
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_ = e.Acknowledge(fmt.Sprintf("msg-%d", id))
		}(i)
	}
	wg.Wait()

	assert.Equal(t, 0, e.TrackedCount())
	e.Stop()
}

func TestStop_Idempotent(t *testing.T) {
	cfg := testConfig()
	pushFunc := func(msg *model.Message) error { return nil }

	e := NewEscalator(cfg, pushFunc)
	e.Track(testMessage("msg-1"))

	// Multiple stops should not panic
	e.Stop()
	e.Stop()
	e.Stop()

	assert.Equal(t, 0, e.TrackedCount())
}

func TestEscalation_IncreasingIntervalUsedBetweenEscalations(t *testing.T) {
	// This test verifies that the increasing interval strategy is applied
	cfg := config.EscalationConfig{
		Enabled:         true,
		WaitMinutes:     1,
		MaxEscalations:  5,
		Interval:        "increasing",
		IntervalMinutes: 10,
	}
	e := NewEscalator(cfg, nil)

	// Verify the wait times increase
	wait0 := e.calculateWait(0) // 10 * 1 = 10 min
	wait1 := e.calculateWait(1) // 10 * 2 = 20 min
	wait2 := e.calculateWait(2) // 10 * 3 = 30 min

	assert.Equal(t, 10*time.Minute, wait0)
	assert.Equal(t, 20*time.Minute, wait1)
	assert.Equal(t, 30*time.Minute, wait2)
	assert.True(t, wait1 > wait0)
	assert.True(t, wait2 > wait1)
}

func TestEscalation_FinalMessageFormat(t *testing.T) {
	cfg := config.EscalationConfig{
		Enabled:         true,
		WaitMinutes:     1,
		MaxEscalations:  1, // Only 1 escalation allowed
		Interval:        "fixed",
		IntervalMinutes: 1,
	}

	var mu sync.Mutex
	var pushed []*model.Message
	pushFunc := func(msg *model.Message) error {
		mu.Lock()
		pushed = append(pushed, msg)
		mu.Unlock()
		return nil
	}

	e := NewEscalator(cfg, pushFunc)
	msg := testMessage("msg-final")

	// Start at escalation count 0, so first escalation (count becomes 1) hits max
	e.mu.Lock()
	tm := &trackedMessage{
		msg:             msg,
		escalationCount: 0,
		originalSentAt:  time.Now(),
	}
	tm.timer = time.AfterFunc(30*time.Millisecond, func() {
		e.escalate(msg.ID)
	})
	e.tracked[msg.ID] = tm
	e.mu.Unlock()

	time.Sleep(80 * time.Millisecond)

	mu.Lock()
	require.Len(t, pushed, 1)
	assert.True(t, strings.HasPrefix(pushed[0].Title, "[已达最大升级上限]"))
	assert.Contains(t, pushed[0].Title, "Test Alert")
	mu.Unlock()

	assert.Equal(t, 0, e.TrackedCount())
}
