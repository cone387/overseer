package repeatpusher

import (
	"sync"
	"testing"
	"time"

	"github.com/overseer/overseer/internal/config"
	"github.com/overseer/overseer/internal/model"
	"github.com/overseer/overseer/internal/store"
	"github.com/overseer/overseer/internal/ws"
)

// mockStore implements store.Store for testing RepeatPusher.
type mockStore struct {
	store.Store // embed to satisfy interface
	messages    map[string]*model.Message
	mu          sync.Mutex
}

func newMockStore() *mockStore {
	return &mockStore{
		messages: make(map[string]*model.Message),
	}
}

func (m *mockStore) GetUnackedMessages(channelNames []string) ([]model.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var result []model.Message
	for _, msg := range m.messages {
		if msg.AckAt != nil {
			continue
		}
		for _, ch := range channelNames {
			if msg.Channel == ch {
				result = append(result, *msg)
				break
			}
		}
	}
	return result, nil
}

func (m *mockStore) GetMessage(id string) (*model.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	msg, ok := m.messages[id]
	if !ok {
		return nil, nil
	}
	// Return a copy
	copy := *msg
	return &copy, nil
}

func (m *mockStore) UpdateRepeatCount(id string, count int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if msg, ok := m.messages[id]; ok {
		msg.RepeatCount = count
	}
	return nil
}

func (m *mockStore) UpdateMessageStatus(id string, status model.PushStatus, failReason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if msg, ok := m.messages[id]; ok {
		msg.Status = status
		msg.FailReason = failReason
	}
	return nil
}

func (m *mockStore) ExpireMessage(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if msg, ok := m.messages[id]; ok {
		msg.Status = model.StatusExpired
	}
	return nil
}

func TestNew(t *testing.T) {
	s := newMockStore()
	hub := ws.NewHub(10)
	channels := []config.Channel{
		{Name: "alerts", RequireAck: true, RepeatInterval: "5m", MaxRepeats: 3},
	}

	rp := New(s, hub, channels)
	if rp == nil {
		t.Fatal("New returned nil")
	}
	if rp.TrackedCount() != 0 {
		t.Errorf("expected 0 tracked, got %d", rp.TrackedCount())
	}
}

func TestScheduleAndCancel(t *testing.T) {
	s := newMockStore()
	hub := ws.NewHub(10)
	channels := []config.Channel{
		{Name: "alerts", RequireAck: true, RepeatInterval: "5m", MaxRepeats: 3},
	}

	rp := New(s, hub, channels)

	msg := &model.Message{
		ID:      "msg-1",
		Channel: "alerts",
		Title:   "Test",
		Body:    "Test body",
		Source:  "test",
	}
	s.messages["msg-1"] = msg

	err := rp.Schedule(msg)
	if err != nil {
		t.Fatalf("Schedule failed: %v", err)
	}
	if rp.TrackedCount() != 1 {
		t.Errorf("expected 1 tracked, got %d", rp.TrackedCount())
	}

	rp.Cancel("msg-1")
	if rp.TrackedCount() != 0 {
		t.Errorf("expected 0 tracked after cancel, got %d", rp.TrackedCount())
	}
}

func TestScheduleIgnoresUnknownChannel(t *testing.T) {
	s := newMockStore()
	hub := ws.NewHub(10)
	channels := []config.Channel{
		{Name: "alerts", RequireAck: true, RepeatInterval: "5m", MaxRepeats: 3},
	}

	rp := New(s, hub, channels)

	msg := &model.Message{
		ID:      "msg-1",
		Channel: "unknown-channel",
		Title:   "Test",
	}

	err := rp.Schedule(msg)
	if err != nil {
		t.Fatalf("Schedule failed: %v", err)
	}
	if rp.TrackedCount() != 0 {
		t.Errorf("expected 0 tracked for unknown channel, got %d", rp.TrackedCount())
	}
}

func TestStartLoadsUnackedMessages(t *testing.T) {
	s := newMockStore()
	hub := ws.NewHub(10)
	channels := []config.Channel{
		{Name: "alerts", RequireAck: true, RepeatInterval: "1m", MaxRepeats: 5},
	}

	// Add unacked messages to the store
	s.messages["msg-1"] = &model.Message{
		ID:          "msg-1",
		Channel:     "alerts",
		Title:       "Alert 1",
		RepeatCount: 0,
		Status:      model.StatusSuccess,
	}
	s.messages["msg-2"] = &model.Message{
		ID:          "msg-2",
		Channel:     "alerts",
		Title:       "Alert 2",
		RepeatCount: 2,
		Status:      model.StatusSuccess,
	}

	rp := New(s, hub, channels)
	err := rp.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer rp.Stop()

	if rp.TrackedCount() != 2 {
		t.Errorf("expected 2 tracked after Start, got %d", rp.TrackedCount())
	}
}

func TestStartSkipsMaxRepeatsReached(t *testing.T) {
	s := newMockStore()
	hub := ws.NewHub(10)
	channels := []config.Channel{
		{Name: "alerts", RequireAck: true, RepeatInterval: "1m", MaxRepeats: 3},
	}

	// Message that has reached max_repeats
	s.messages["msg-1"] = &model.Message{
		ID:          "msg-1",
		Channel:     "alerts",
		Title:       "Alert 1",
		RepeatCount: 3,
		Status:      model.StatusSuccess,
	}

	rp := New(s, hub, channels)
	err := rp.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer rp.Stop()

	if rp.TrackedCount() != 0 {
		t.Errorf("expected 0 tracked (max_repeats reached), got %d", rp.TrackedCount())
	}
}

func TestStartSkipsExpiredMessages(t *testing.T) {
	s := newMockStore()
	hub := ws.NewHub(10)
	channels := []config.Channel{
		{Name: "alerts", RequireAck: true, RepeatInterval: "1m", MaxRepeats: 5},
	}

	past := time.Now().Add(-1 * time.Hour)
	s.messages["msg-1"] = &model.Message{
		ID:          "msg-1",
		Channel:     "alerts",
		Title:       "Expired Alert",
		RepeatCount: 0,
		ExpiresAt:   &past,
		Status:      model.StatusSuccess,
	}

	rp := New(s, hub, channels)
	err := rp.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer rp.Stop()

	if rp.TrackedCount() != 0 {
		t.Errorf("expected 0 tracked (expired), got %d", rp.TrackedCount())
	}
}

func TestStartSchedulesSnoozedMessages(t *testing.T) {
	s := newMockStore()
	hub := ws.NewHub(10)
	channels := []config.Channel{
		{Name: "alerts", RequireAck: true, RepeatInterval: "1m", MaxRepeats: 5},
	}

	future := time.Now().Add(30 * time.Minute)
	s.messages["msg-1"] = &model.Message{
		ID:          "msg-1",
		Channel:     "alerts",
		Title:       "Snoozed Alert",
		RepeatCount: 0,
		SnoozeUntil: &future,
		Status:      model.StatusSuccess,
	}

	rp := New(s, hub, channels)
	err := rp.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer rp.Stop()

	// Snoozed messages are still tracked (with a snooze timer)
	if rp.TrackedCount() != 1 {
		t.Errorf("expected 1 tracked (snoozed), got %d", rp.TrackedCount())
	}
}

func TestScheduleSnooze(t *testing.T) {
	s := newMockStore()
	hub := ws.NewHub(10)
	channels := []config.Channel{
		{Name: "alerts", RequireAck: true, RepeatInterval: "5m", MaxRepeats: 3},
	}

	rp := New(s, hub, channels)

	msg := &model.Message{
		ID:      "msg-1",
		Channel: "alerts",
		Title:   "Test",
	}
	s.messages["msg-1"] = msg

	// Schedule the message first
	_ = rp.Schedule(msg)
	if rp.TrackedCount() != 1 {
		t.Fatalf("expected 1 tracked, got %d", rp.TrackedCount())
	}

	// Snooze it
	snoozeUntil := time.Now().Add(1 * time.Hour)
	rp.ScheduleSnooze("msg-1", snoozeUntil)

	// Should still be tracked (with snooze timer)
	if rp.TrackedCount() != 1 {
		t.Errorf("expected 1 tracked after snooze, got %d", rp.TrackedCount())
	}
}

func TestStop(t *testing.T) {
	s := newMockStore()
	hub := ws.NewHub(10)
	channels := []config.Channel{
		{Name: "alerts", RequireAck: true, RepeatInterval: "5m", MaxRepeats: 3},
	}

	rp := New(s, hub, channels)

	msg := &model.Message{
		ID:      "msg-1",
		Channel: "alerts",
		Title:   "Test",
	}
	s.messages["msg-1"] = msg

	_ = rp.Schedule(msg)
	if rp.TrackedCount() != 1 {
		t.Fatalf("expected 1 tracked, got %d", rp.TrackedCount())
	}

	rp.Stop()
	if rp.TrackedCount() != 0 {
		t.Errorf("expected 0 tracked after Stop, got %d", rp.TrackedCount())
	}
}

func TestOnTimerFiredAckedMessage(t *testing.T) {
	s := newMockStore()
	hub := ws.NewHub(10)
	go hub.Run()
	channels := []config.Channel{
		{Name: "alerts", RequireAck: true, RepeatInterval: "10ms", MaxRepeats: 5},
	}

	now := time.Now()
	ackTime := now.Add(-5 * time.Minute)
	s.messages["msg-1"] = &model.Message{
		ID:      "msg-1",
		Channel: "alerts",
		Title:   "Acked Alert",
		Source:  "test",
		AckAt:   &ackTime,
		Status:  model.StatusSuccess,
	}

	rp := New(s, hub, channels)

	// Manually schedule — the timer will fire and find the message is acked
	msg := &model.Message{
		ID:      "msg-1",
		Channel: "alerts",
		Title:   "Acked Alert",
		Source:  "test",
	}
	_ = rp.Schedule(msg)

	// Wait for timer to fire
	time.Sleep(50 * time.Millisecond)

	// Should not be tracked anymore (timer fired and found ack)
	if rp.TrackedCount() != 0 {
		t.Errorf("expected 0 tracked (acked message), got %d", rp.TrackedCount())
	}

	// Repeat count should not have been incremented
	s.mu.Lock()
	if s.messages["msg-1"].RepeatCount != 0 {
		t.Errorf("expected repeat_count 0 for acked message, got %d", s.messages["msg-1"].RepeatCount)
	}
	s.mu.Unlock()
}

func TestOnTimerFiredMaxRepeats(t *testing.T) {
	s := newMockStore()
	hub := ws.NewHub(10)
	go hub.Run()
	channels := []config.Channel{
		{Name: "alerts", RequireAck: true, RepeatInterval: "10ms", MaxRepeats: 2},
	}

	s.messages["msg-1"] = &model.Message{
		ID:          "msg-1",
		Channel:     "alerts",
		Title:       "Alert",
		Source:      "test",
		RepeatCount: 1, // one more repeat will hit max
		Status:      model.StatusSuccess,
		ReceivedAt:  time.Now(),
	}

	rp := New(s, hub, channels)

	msg := &model.Message{
		ID:      "msg-1",
		Channel: "alerts",
	}
	_ = rp.Schedule(msg)

	// Wait for timer to fire
	time.Sleep(50 * time.Millisecond)

	// Should be marked as expired_unacked
	s.mu.Lock()
	if s.messages["msg-1"].Status != model.StatusExpiredUnacked {
		t.Errorf("expected status expired_unacked, got %s", s.messages["msg-1"].Status)
	}
	s.mu.Unlock()

	// Should not be tracked anymore
	if rp.TrackedCount() != 0 {
		t.Errorf("expected 0 tracked after max_repeats, got %d", rp.TrackedCount())
	}
}
