package store

import (
	"testing"
	"time"

	"github.com/overseer/overseer/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeReminder(id string) *model.Reminder {
	now := time.Now().Truncate(time.Second)
	trigger := now.Add(1 * time.Hour)
	return &model.Reminder{
		ID:         id,
		Title:      "Test Reminder " + id,
		Body:       "Body for " + id,
		Channel:    "default",
		TriggerAt:  trigger,
		RepeatType: model.RepeatOnce,
		RepeatRule: "",
		Status:     "active",
		NextTrigger: &trigger,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

func TestCreateReminder(t *testing.T) {
	s := newTestStore(t)
	r := makeReminder("rem-001")

	err := s.CreateReminder(r)
	require.NoError(t, err)

	// Verify it was inserted by listing.
	reminders, err := s.ListReminders(ReminderFilter{})
	require.NoError(t, err)
	require.Len(t, reminders, 1)
	assert.Equal(t, r.ID, reminders[0].ID)
	assert.Equal(t, r.Title, reminders[0].Title)
	assert.Equal(t, r.Body, reminders[0].Body)
	assert.Equal(t, r.Channel, reminders[0].Channel)
	assert.Equal(t, r.RepeatType, reminders[0].RepeatType)
	assert.Equal(t, r.Status, reminders[0].Status)
}

func TestCreateReminder_DuplicateID(t *testing.T) {
	s := newTestStore(t)
	r := makeReminder("rem-dup")

	require.NoError(t, s.CreateReminder(r))
	err := s.CreateReminder(r)
	assert.Error(t, err)
}

func TestUpdateReminder(t *testing.T) {
	s := newTestStore(t)
	r := makeReminder("rem-002")
	require.NoError(t, s.CreateReminder(r))

	// Modify fields.
	r.Title = "Updated Title"
	r.Body = "Updated Body"
	r.Channel = "urgent"
	r.RepeatType = model.RepeatDaily
	r.RepeatRule = ""
	newTrigger := r.TriggerAt.Add(24 * time.Hour)
	r.NextTrigger = &newTrigger

	err := s.UpdateReminder(r)
	require.NoError(t, err)

	// Verify update.
	reminders, err := s.ListReminders(ReminderFilter{})
	require.NoError(t, err)
	require.Len(t, reminders, 1)
	assert.Equal(t, "Updated Title", reminders[0].Title)
	assert.Equal(t, "Updated Body", reminders[0].Body)
	assert.Equal(t, "urgent", reminders[0].Channel)
	assert.Equal(t, model.RepeatDaily, reminders[0].RepeatType)
}

func TestUpdateReminder_NotFound(t *testing.T) {
	s := newTestStore(t)

	r := makeReminder("nonexistent")
	err := s.UpdateReminder(r)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestCancelReminder(t *testing.T) {
	s := newTestStore(t)
	r := makeReminder("rem-003")
	require.NoError(t, s.CreateReminder(r))

	err := s.CancelReminder("rem-003")
	require.NoError(t, err)

	// Verify status changed.
	reminders, err := s.ListReminders(ReminderFilter{Status: "cancelled"})
	require.NoError(t, err)
	require.Len(t, reminders, 1)
	assert.Equal(t, "cancelled", reminders[0].Status)
}

func TestCancelReminder_NotFound(t *testing.T) {
	s := newTestStore(t)

	err := s.CancelReminder("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestListReminders_All(t *testing.T) {
	s := newTestStore(t)

	r1 := makeReminder("rem-a")
	r2 := makeReminder("rem-b")
	r2.Status = "cancelled"
	require.NoError(t, s.CreateReminder(r1))
	require.NoError(t, s.CreateReminder(r2))

	reminders, err := s.ListReminders(ReminderFilter{})
	require.NoError(t, err)
	assert.Len(t, reminders, 2)
}

func TestListReminders_FilterByStatus(t *testing.T) {
	s := newTestStore(t)

	r1 := makeReminder("rem-active")
	r1.Status = "active"
	r2 := makeReminder("rem-cancelled")
	r2.Status = "cancelled"
	r3 := makeReminder("rem-completed")
	r3.Status = "completed"
	require.NoError(t, s.CreateReminder(r1))
	require.NoError(t, s.CreateReminder(r2))
	require.NoError(t, s.CreateReminder(r3))

	active, err := s.ListReminders(ReminderFilter{Status: "active"})
	require.NoError(t, err)
	assert.Len(t, active, 1)
	assert.Equal(t, "rem-active", active[0].ID)

	cancelled, err := s.ListReminders(ReminderFilter{Status: "cancelled"})
	require.NoError(t, err)
	assert.Len(t, cancelled, 1)
	assert.Equal(t, "rem-cancelled", cancelled[0].ID)
}

func TestGetActiveReminders(t *testing.T) {
	s := newTestStore(t)

	r1 := makeReminder("rem-act1")
	r1.Status = "active"
	r2 := makeReminder("rem-act2")
	r2.Status = "active"
	r3 := makeReminder("rem-can")
	r3.Status = "cancelled"
	require.NoError(t, s.CreateReminder(r1))
	require.NoError(t, s.CreateReminder(r2))
	require.NoError(t, s.CreateReminder(r3))

	active, err := s.GetActiveReminders()
	require.NoError(t, err)
	assert.Len(t, active, 2)
	for _, r := range active {
		assert.Equal(t, "active", r.Status)
	}
}

func TestGetActiveReminders_Empty(t *testing.T) {
	s := newTestStore(t)

	active, err := s.GetActiveReminders()
	require.NoError(t, err)
	assert.Empty(t, active)
}

func TestUpdateNextTrigger(t *testing.T) {
	s := newTestStore(t)
	r := makeReminder("rem-004")
	require.NoError(t, s.CreateReminder(r))

	newNext := time.Now().Add(48 * time.Hour).Truncate(time.Second)
	err := s.UpdateNextTrigger("rem-004", newNext)
	require.NoError(t, err)

	// Verify the next_trigger was updated.
	reminders, err := s.ListReminders(ReminderFilter{})
	require.NoError(t, err)
	require.Len(t, reminders, 1)
	require.NotNil(t, reminders[0].NextTrigger)
	assert.WithinDuration(t, newNext, *reminders[0].NextTrigger, time.Second)
}

func TestUpdateNextTrigger_NotFound(t *testing.T) {
	s := newTestStore(t)

	err := s.UpdateNextTrigger("nonexistent", time.Now())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestCreateReminder_NilOptionalFields(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().Truncate(time.Second)

	r := &model.Reminder{
		ID:         "rem-nil",
		Title:      "Minimal Reminder",
		Body:       "",
		Channel:    "default",
		TriggerAt:  now.Add(time.Hour),
		RepeatType: model.RepeatOnce,
		Status:     "active",
		NextTrigger: nil,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	require.NoError(t, s.CreateReminder(r))

	reminders, err := s.ListReminders(ReminderFilter{})
	require.NoError(t, err)
	require.Len(t, reminders, 1)
	assert.Nil(t, reminders[0].NextTrigger)
	assert.Nil(t, reminders[0].LastTriggered)
	assert.Empty(t, reminders[0].Body)
}

func TestGetActiveReminders_OrderedByNextTrigger(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().Truncate(time.Second)

	// Create reminders with different next_trigger times.
	r1 := makeReminder("rem-later")
	later := now.Add(3 * time.Hour)
	r1.NextTrigger = &later

	r2 := makeReminder("rem-sooner")
	sooner := now.Add(1 * time.Hour)
	r2.NextTrigger = &sooner

	require.NoError(t, s.CreateReminder(r1))
	require.NoError(t, s.CreateReminder(r2))

	active, err := s.GetActiveReminders()
	require.NoError(t, err)
	require.Len(t, active, 2)
	// Should be ordered by next_trigger ASC.
	assert.Equal(t, "rem-sooner", active[0].ID)
	assert.Equal(t, "rem-later", active[1].ID)
}
