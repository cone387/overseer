package store

import (
	"fmt"
	"testing"
	"time"

	"github.com/overseer/overseer/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	store, err := NewSQLiteStore(":memory:")
	require.NoError(t, err)
	require.NoError(t, store.Migrate())
	t.Cleanup(func() { store.Close() })
	return store
}

func TestNewSQLiteStore_InMemory(t *testing.T) {
	store, err := NewSQLiteStore(":memory:")
	require.NoError(t, err)
	defer store.Close()

	assert.NotNil(t, store)
}

func TestSQLiteStore_Migrate(t *testing.T) {
	store, err := NewSQLiteStore(":memory:")
	require.NoError(t, err)
	defer store.Close()

	err = store.Migrate()
	require.NoError(t, err)

	// Verify tables exist by querying sqlite_master.
	rows, err := store.db.Query(`SELECT name FROM sqlite_master WHERE type='table' ORDER BY name`)
	require.NoError(t, err)
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		tables = append(tables, name)
	}
	require.NoError(t, rows.Err())

	assert.Contains(t, tables, "messages")
	assert.Contains(t, tables, "reminders")
	assert.Contains(t, tables, "push_results")
}

func TestSQLiteStore_Migrate_Idempotent(t *testing.T) {
	store, err := NewSQLiteStore(":memory:")
	require.NoError(t, err)
	defer store.Close()

	// Running Migrate twice should not error.
	require.NoError(t, store.Migrate())
	require.NoError(t, store.Migrate())
}

func TestSQLiteStore_Migrate_Indexes(t *testing.T) {
	store, err := NewSQLiteStore(":memory:")
	require.NoError(t, err)
	defer store.Close()

	require.NoError(t, store.Migrate())

	// Verify indexes exist.
	rows, err := store.db.Query(`SELECT name FROM sqlite_master WHERE type='index' AND name LIKE 'idx_%' ORDER BY name`)
	require.NoError(t, err)
	defer rows.Close()

	var indexes []string
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		indexes = append(indexes, name)
	}
	require.NoError(t, rows.Err())

	expectedIndexes := []string{
		"idx_messages_channel",
		"idx_messages_status",
		"idx_messages_received_at",
		"idx_reminders_status",
		"idx_reminders_next_trigger",
		"idx_push_results_message",
	}
	for _, idx := range expectedIndexes {
		assert.Contains(t, indexes, idx)
	}
}

func TestSQLiteStore_Close(t *testing.T) {
	store, err := NewSQLiteStore(":memory:")
	require.NoError(t, err)

	err = store.Close()
	require.NoError(t, err)

	// After close, operations should fail.
	err = store.db.Ping()
	assert.Error(t, err)
}

// Verify SQLiteStore implements the Store interface at compile time.
var _ Store = (*SQLiteStore)(nil)

// --- SaveMessage tests ---

func TestSQLiteStore_SaveMessage(t *testing.T) {
	store := newTestStore(t)

	now := time.Now().Truncate(time.Second)
	msg := &model.Message{
		ID:         "msg-001",
		Source:     "github",
		Channel:    "urgent",
		Title:      "CI Failed",
		Body:       "Build #42 failed",
		Extra:      map[string]string{"repo": "overseer", "branch": "main"},
		Status:     model.StatusPending,
		RetryCount: 0,
		ReceivedAt: now,
	}

	err := store.SaveMessage(msg)
	require.NoError(t, err)

	// Verify by querying back.
	result, err := store.QueryMessages(MessageFilter{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Len(t, result.Data, 1)

	saved := result.Data[0]
	assert.Equal(t, msg.ID, saved.ID)
	assert.Equal(t, msg.Source, saved.Source)
	assert.Equal(t, msg.Channel, saved.Channel)
	assert.Equal(t, msg.Title, saved.Title)
	assert.Equal(t, msg.Body, saved.Body)
	assert.Equal(t, msg.Extra, saved.Extra)
	assert.Equal(t, model.StatusPending, saved.Status)
	assert.Equal(t, 0, saved.RetryCount)
	assert.Nil(t, saved.PushedAt)
}

func TestSQLiteStore_SaveMessage_NoExtra(t *testing.T) {
	store := newTestStore(t)

	msg := &model.Message{
		ID:         "msg-002",
		Source:     "grafana",
		Channel:    "monitor",
		Title:      "Alert",
		Body:       "CPU high",
		Status:     model.StatusPending,
		ReceivedAt: time.Now().Truncate(time.Second),
	}

	err := store.SaveMessage(msg)
	require.NoError(t, err)

	result, err := store.QueryMessages(MessageFilter{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Len(t, result.Data, 1)
	assert.Nil(t, result.Data[0].Extra)
}

func TestSQLiteStore_SaveMessage_DuplicateID(t *testing.T) {
	store := newTestStore(t)

	msg := &model.Message{
		ID:         "msg-dup",
		Source:     "test",
		Channel:    "default",
		Title:      "Test",
		Body:       "Body",
		Status:     model.StatusPending,
		ReceivedAt: time.Now(),
	}

	require.NoError(t, store.SaveMessage(msg))
	err := store.SaveMessage(msg)
	assert.Error(t, err) // PRIMARY KEY constraint
}

// --- UpdateMessageStatus tests ---

func TestSQLiteStore_UpdateMessageStatus_Success(t *testing.T) {
	store := newTestStore(t)

	msg := &model.Message{
		ID:         "msg-upd-1",
		Source:     "test",
		Channel:    "default",
		Title:      "Test",
		Body:       "Body",
		Status:     model.StatusPending,
		ReceivedAt: time.Now().Truncate(time.Second),
	}
	require.NoError(t, store.SaveMessage(msg))

	err := store.UpdateMessageStatus("msg-upd-1", model.StatusSuccess, "")
	require.NoError(t, err)

	result, err := store.QueryMessages(MessageFilter{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Len(t, result.Data, 1)

	updated := result.Data[0]
	assert.Equal(t, model.StatusSuccess, updated.Status)
	assert.Equal(t, 1, updated.RetryCount)
	assert.NotNil(t, updated.PushedAt)
}

func TestSQLiteStore_UpdateMessageStatus_Failed(t *testing.T) {
	store := newTestStore(t)

	msg := &model.Message{
		ID:         "msg-upd-2",
		Source:     "test",
		Channel:    "default",
		Title:      "Test",
		Body:       "Body",
		Status:     model.StatusPending,
		ReceivedAt: time.Now().Truncate(time.Second),
	}
	require.NoError(t, store.SaveMessage(msg))

	err := store.UpdateMessageStatus("msg-upd-2", model.StatusFailed, "connection timeout")
	require.NoError(t, err)

	result, err := store.QueryMessages(MessageFilter{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Len(t, result.Data, 1)

	updated := result.Data[0]
	assert.Equal(t, model.StatusFailed, updated.Status)
	assert.Equal(t, "connection timeout", updated.FailReason)
	assert.Equal(t, 1, updated.RetryCount)
	assert.Nil(t, updated.PushedAt)
}

func TestSQLiteStore_UpdateMessageStatus_IncrementRetryCount(t *testing.T) {
	store := newTestStore(t)

	msg := &model.Message{
		ID:         "msg-retry",
		Source:     "test",
		Channel:    "default",
		Title:      "Test",
		Body:       "Body",
		Status:     model.StatusPending,
		ReceivedAt: time.Now().Truncate(time.Second),
	}
	require.NoError(t, store.SaveMessage(msg))

	// Simulate multiple retries.
	require.NoError(t, store.UpdateMessageStatus("msg-retry", model.StatusFailed, "timeout"))
	require.NoError(t, store.UpdateMessageStatus("msg-retry", model.StatusFailed, "timeout"))
	require.NoError(t, store.UpdateMessageStatus("msg-retry", model.StatusSuccess, ""))

	result, err := store.QueryMessages(MessageFilter{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Len(t, result.Data, 1)
	assert.Equal(t, 3, result.Data[0].RetryCount)
}

// --- QueryMessages tests ---

func TestSQLiteStore_QueryMessages_FilterByChannel(t *testing.T) {
	store := newTestStore(t)

	now := time.Now().Truncate(time.Second)
	msgs := []*model.Message{
		{ID: "m1", Source: "s", Channel: "urgent", Title: "T1", Body: "B1", Status: model.StatusPending, ReceivedAt: now},
		{ID: "m2", Source: "s", Channel: "default", Title: "T2", Body: "B2", Status: model.StatusPending, ReceivedAt: now.Add(time.Second)},
		{ID: "m3", Source: "s", Channel: "urgent", Title: "T3", Body: "B3", Status: model.StatusPending, ReceivedAt: now.Add(2 * time.Second)},
	}
	for _, m := range msgs {
		require.NoError(t, store.SaveMessage(m))
	}

	result, err := store.QueryMessages(MessageFilter{Channel: "urgent", Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, 2, result.Total)
	assert.Len(t, result.Data, 2)
	for _, m := range result.Data {
		assert.Equal(t, "urgent", m.Channel)
	}
}

func TestSQLiteStore_QueryMessages_FilterByStatus(t *testing.T) {
	store := newTestStore(t)

	now := time.Now().Truncate(time.Second)
	msgs := []*model.Message{
		{ID: "m1", Source: "s", Channel: "ch", Title: "T1", Body: "B1", Status: model.StatusPending, ReceivedAt: now},
		{ID: "m2", Source: "s", Channel: "ch", Title: "T2", Body: "B2", Status: model.StatusSuccess, ReceivedAt: now.Add(time.Second)},
		{ID: "m3", Source: "s", Channel: "ch", Title: "T3", Body: "B3", Status: model.StatusFailed, ReceivedAt: now.Add(2 * time.Second)},
	}
	for _, m := range msgs {
		require.NoError(t, store.SaveMessage(m))
	}

	result, err := store.QueryMessages(MessageFilter{Status: model.StatusSuccess, Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Total)
	assert.Len(t, result.Data, 1)
	assert.Equal(t, "m2", result.Data[0].ID)
}

func TestSQLiteStore_QueryMessages_FilterByTimeRange(t *testing.T) {
	store := newTestStore(t)

	base := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	msgs := []*model.Message{
		{ID: "m1", Source: "s", Channel: "ch", Title: "T1", Body: "B1", Status: model.StatusPending, ReceivedAt: base},
		{ID: "m2", Source: "s", Channel: "ch", Title: "T2", Body: "B2", Status: model.StatusPending, ReceivedAt: base.Add(time.Hour)},
		{ID: "m3", Source: "s", Channel: "ch", Title: "T3", Body: "B3", Status: model.StatusPending, ReceivedAt: base.Add(2 * time.Hour)},
	}
	for _, m := range msgs {
		require.NoError(t, store.SaveMessage(m))
	}

	result, err := store.QueryMessages(MessageFilter{
		From:     base.Add(30 * time.Minute),
		To:       base.Add(90 * time.Minute),
		Page:     1,
		PageSize: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Total)
	assert.Equal(t, "m2", result.Data[0].ID)
}

func TestSQLiteStore_QueryMessages_TimeDescOrder(t *testing.T) {
	store := newTestStore(t)

	base := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	msgs := []*model.Message{
		{ID: "m1", Source: "s", Channel: "ch", Title: "T1", Body: "B1", Status: model.StatusPending, ReceivedAt: base},
		{ID: "m2", Source: "s", Channel: "ch", Title: "T2", Body: "B2", Status: model.StatusPending, ReceivedAt: base.Add(time.Hour)},
		{ID: "m3", Source: "s", Channel: "ch", Title: "T3", Body: "B3", Status: model.StatusPending, ReceivedAt: base.Add(2 * time.Hour)},
	}
	for _, m := range msgs {
		require.NoError(t, store.SaveMessage(m))
	}

	result, err := store.QueryMessages(MessageFilter{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Len(t, result.Data, 3)
	// Most recent first.
	assert.Equal(t, "m3", result.Data[0].ID)
	assert.Equal(t, "m2", result.Data[1].ID)
	assert.Equal(t, "m1", result.Data[2].ID)
}

func TestSQLiteStore_QueryMessages_Pagination(t *testing.T) {
	store := newTestStore(t)

	base := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		msg := &model.Message{
			ID:         fmt.Sprintf("m%d", i),
			Source:     "s",
			Channel:    "ch",
			Title:      fmt.Sprintf("T%d", i),
			Body:       "B",
			Status:     model.StatusPending,
			ReceivedAt: base.Add(time.Duration(i) * time.Minute),
		}
		require.NoError(t, store.SaveMessage(msg))
	}

	// Page 1, size 2.
	result, err := store.QueryMessages(MessageFilter{Page: 1, PageSize: 2})
	require.NoError(t, err)
	assert.Equal(t, 5, result.Total)
	assert.Equal(t, 1, result.Page)
	assert.Equal(t, 2, result.PageSize)
	assert.Len(t, result.Data, 2)
	assert.Equal(t, "m4", result.Data[0].ID) // Most recent first.
	assert.Equal(t, "m3", result.Data[1].ID)

	// Page 2, size 2.
	result, err = store.QueryMessages(MessageFilter{Page: 2, PageSize: 2})
	require.NoError(t, err)
	assert.Equal(t, 5, result.Total)
	assert.Len(t, result.Data, 2)
	assert.Equal(t, "m2", result.Data[0].ID)
	assert.Equal(t, "m1", result.Data[1].ID)

	// Page 3, size 2 (last page with 1 item).
	result, err = store.QueryMessages(MessageFilter{Page: 3, PageSize: 2})
	require.NoError(t, err)
	assert.Equal(t, 5, result.Total)
	assert.Len(t, result.Data, 1)
	assert.Equal(t, "m0", result.Data[0].ID)
}

func TestSQLiteStore_QueryMessages_EmptyResult(t *testing.T) {
	store := newTestStore(t)

	result, err := store.QueryMessages(MessageFilter{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, 0, result.Total)
	assert.Empty(t, result.Data)
}

func TestSQLiteStore_QueryMessages_DefaultPagination(t *testing.T) {
	store := newTestStore(t)

	msg := &model.Message{
		ID:         "m1",
		Source:     "s",
		Channel:    "ch",
		Title:      "T",
		Body:       "B",
		Status:     model.StatusPending,
		ReceivedAt: time.Now(),
	}
	require.NoError(t, store.SaveMessage(msg))

	// Page=0 and PageSize=0 should default to page=1, pageSize=20.
	result, err := store.QueryMessages(MessageFilter{})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Page)
	assert.Equal(t, 20, result.PageSize)
}

// --- GetChannelStats tests ---

func TestSQLiteStore_GetChannelStats(t *testing.T) {
	store := newTestStore(t)

	base := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	msgs := []*model.Message{
		{ID: "m1", Source: "s", Channel: "urgent", Title: "T", Body: "B", Status: model.StatusSuccess, ReceivedAt: base},
		{ID: "m2", Source: "s", Channel: "urgent", Title: "T", Body: "B", Status: model.StatusSuccess, ReceivedAt: base.Add(time.Minute)},
		{ID: "m3", Source: "s", Channel: "urgent", Title: "T", Body: "B", Status: model.StatusFailed, ReceivedAt: base.Add(2 * time.Minute)},
		{ID: "m4", Source: "s", Channel: "default", Title: "T", Body: "B", Status: model.StatusSuccess, ReceivedAt: base.Add(3 * time.Minute)},
		{ID: "m5", Source: "s", Channel: "default", Title: "T", Body: "B", Status: model.StatusPending, ReceivedAt: base.Add(4 * time.Minute)},
	}
	for _, m := range msgs {
		require.NoError(t, store.SaveMessage(m))
	}

	stats, err := store.GetChannelStats(base.Add(-time.Hour), base.Add(time.Hour))
	require.NoError(t, err)
	require.Len(t, stats, 2)

	// Build a map for easier assertion.
	statsMap := make(map[string]model.ChannelStats)
	for _, s := range stats {
		statsMap[s.Channel] = s
	}

	urgent := statsMap["urgent"]
	assert.Equal(t, 3, urgent.Total)
	assert.Equal(t, 2, urgent.SuccessCount)
	assert.Equal(t, 1, urgent.FailedCount)
	assert.InDelta(t, 66.67, urgent.SuccessPercent, 0.01)
	assert.InDelta(t, 33.33, urgent.FailedPercent, 0.01)

	def := statsMap["default"]
	assert.Equal(t, 2, def.Total)
	assert.Equal(t, 1, def.SuccessCount)
	assert.Equal(t, 0, def.FailedCount)
	assert.InDelta(t, 50.0, def.SuccessPercent, 0.01)
	assert.InDelta(t, 0.0, def.FailedPercent, 0.01)
}

func TestSQLiteStore_GetChannelStats_TimeRangeFilter(t *testing.T) {
	store := newTestStore(t)

	base := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	msgs := []*model.Message{
		{ID: "m1", Source: "s", Channel: "ch", Title: "T", Body: "B", Status: model.StatusSuccess, ReceivedAt: base},
		{ID: "m2", Source: "s", Channel: "ch", Title: "T", Body: "B", Status: model.StatusSuccess, ReceivedAt: base.Add(2 * time.Hour)},
	}
	for _, m := range msgs {
		require.NoError(t, store.SaveMessage(m))
	}

	// Only include the first message.
	stats, err := store.GetChannelStats(base.Add(-time.Minute), base.Add(time.Minute))
	require.NoError(t, err)
	require.Len(t, stats, 1)
	assert.Equal(t, 1, stats[0].Total)
}

func TestSQLiteStore_GetChannelStats_Empty(t *testing.T) {
	store := newTestStore(t)

	stats, err := store.GetChannelStats(time.Now().Add(-time.Hour), time.Now())
	require.NoError(t, err)
	assert.Empty(t, stats)
}
