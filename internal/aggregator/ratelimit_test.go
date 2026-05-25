package aggregator

import (
	"fmt"
	"testing"
	"time"

	"github.com/overseer/overseer/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestMessage(title string) *model.Message {
	return &model.Message{
		ID:         fmt.Sprintf("msg-%s", title),
		Title:      title,
		Body:       "test body",
		Channel:    "test-channel",
		ReceivedAt: time.Now(),
	}
}

func TestNewRateLimiter_Defaults(t *testing.T) {
	rl := NewRateLimiter(0, 0, 0)
	assert.Equal(t, 3600, rl.rateWindow)
	assert.Equal(t, 30, rl.rateLimit)
	assert.Equal(t, 100, rl.maxPending)
}

func TestNewRateLimiter_CustomValues(t *testing.T) {
	rl := NewRateLimiter(60, 5, 10)
	assert.Equal(t, 60, rl.rateWindow)
	assert.Equal(t, 5, rl.rateLimit)
	assert.Equal(t, 10, rl.maxPending)
}

func TestAllow_UnderLimit(t *testing.T) {
	rl := NewRateLimiter(3600, 3, 100)

	for i := 0; i < 3; i++ {
		msg := newTestMessage(fmt.Sprintf("msg-%d", i))
		result := rl.Allow("ch1", msg)
		assert.True(t, result.Allowed, "message %d should be allowed", i)
		assert.Empty(t, result.Summary)
	}
	assert.Equal(t, 3, rl.Count("ch1"))
}

func TestAllow_AtLimit_QueuesMessages(t *testing.T) {
	rl := NewRateLimiter(3600, 2, 100)

	// First two messages are allowed
	r1 := rl.Allow("ch1", newTestMessage("msg-1"))
	assert.True(t, r1.Allowed)

	r2 := rl.Allow("ch1", newTestMessage("msg-2"))
	assert.True(t, r2.Allowed)

	// Third message should be rate limited
	r3 := rl.Allow("ch1", newTestMessage("msg-3"))
	assert.False(t, r3.Allowed)
	assert.Empty(t, r3.Summary)

	assert.Equal(t, 1, rl.PendingCount("ch1"))
}

func TestAllow_PendingExceedsMax_GeneratesSummary(t *testing.T) {
	maxPending := 3
	rl := NewRateLimiter(3600, 1, maxPending)

	// First message is allowed
	r := rl.Allow("ch1", newTestMessage("allowed"))
	assert.True(t, r.Allowed)

	// Next messages are queued (up to maxPending)
	for i := 0; i < maxPending; i++ {
		r = rl.Allow("ch1", newTestMessage(fmt.Sprintf("pending-%d", i)))
		assert.False(t, r.Allowed)
	}

	// At this point we have maxPending messages queued.
	// The next one should trigger summary (exceeds maxPending).
	r = rl.Allow("ch1", newTestMessage("overflow"))
	assert.False(t, r.Allowed)
	assert.NotEmpty(t, r.Summary)
	assert.Contains(t, r.Summary, "频率限制摘要")
	assert.Contains(t, r.Summary, "ch1")

	// Queue should be cleared after summary
	assert.Equal(t, 0, rl.PendingCount("ch1"))
}

func TestAllow_WindowExpiry_ResetsCount(t *testing.T) {
	rl := NewRateLimiter(60, 2, 100) // 60 second window

	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	rl.nowFunc = func() time.Time { return now }

	// Use up the limit
	rl.Allow("ch1", newTestMessage("msg-1"))
	rl.Allow("ch1", newTestMessage("msg-2"))

	// Should be rate limited
	r := rl.Allow("ch1", newTestMessage("msg-3"))
	assert.False(t, r.Allowed)

	// Advance time past the window
	now = now.Add(61 * time.Second)

	// Should be allowed again
	r = rl.Allow("ch1", newTestMessage("msg-4"))
	assert.True(t, r.Allowed)
	assert.Equal(t, 1, rl.Count("ch1"))
}

func TestAllow_DifferentChannels_Independent(t *testing.T) {
	rl := NewRateLimiter(3600, 2, 100)

	// Fill up ch1
	rl.Allow("ch1", newTestMessage("ch1-1"))
	rl.Allow("ch1", newTestMessage("ch1-2"))
	r := rl.Allow("ch1", newTestMessage("ch1-3"))
	assert.False(t, r.Allowed)

	// ch2 should still be available
	r = rl.Allow("ch2", newTestMessage("ch2-1"))
	assert.True(t, r.Allowed)
}

func TestFlush_ReturnsPendingInOrder(t *testing.T) {
	rl := NewRateLimiter(3600, 1, 100)

	// Allow one message
	rl.Allow("ch1", newTestMessage("allowed"))

	// Queue several messages
	msgs := []string{"first", "second", "third"}
	for _, title := range msgs {
		rl.Allow("ch1", newTestMessage(title))
	}

	// Flush should return messages in receive order
	flushed := rl.Flush("ch1")
	require.Len(t, flushed, 3)
	assert.Equal(t, "first", flushed[0].Title)
	assert.Equal(t, "second", flushed[1].Title)
	assert.Equal(t, "third", flushed[2].Title)

	// After flush, pending should be empty and count reset
	assert.Equal(t, 0, rl.PendingCount("ch1"))
	assert.Equal(t, 0, rl.Count("ch1"))
}

func TestFlush_EmptyChannel_ReturnsNil(t *testing.T) {
	rl := NewRateLimiter(3600, 10, 100)

	flushed := rl.Flush("nonexistent")
	assert.Nil(t, flushed)
}

func TestFlush_NoPending_ReturnsNil(t *testing.T) {
	rl := NewRateLimiter(3600, 10, 100)

	// Allow a message but don't exceed limit
	rl.Allow("ch1", newTestMessage("msg-1"))

	flushed := rl.Flush("ch1")
	assert.Nil(t, flushed)
}

func TestPendingCount(t *testing.T) {
	rl := NewRateLimiter(3600, 1, 100)

	assert.Equal(t, 0, rl.PendingCount("ch1"))

	rl.Allow("ch1", newTestMessage("allowed"))
	assert.Equal(t, 0, rl.PendingCount("ch1"))

	rl.Allow("ch1", newTestMessage("pending-1"))
	assert.Equal(t, 1, rl.PendingCount("ch1"))

	rl.Allow("ch1", newTestMessage("pending-2"))
	assert.Equal(t, 2, rl.PendingCount("ch1"))
}

func TestCount_NonexistentChannel(t *testing.T) {
	rl := NewRateLimiter(3600, 10, 100)
	assert.Equal(t, 0, rl.Count("nonexistent"))
}

func TestCount_AfterWindowExpiry(t *testing.T) {
	rl := NewRateLimiter(60, 10, 100)

	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	rl.nowFunc = func() time.Time { return now }

	rl.Allow("ch1", newTestMessage("msg-1"))
	assert.Equal(t, 1, rl.Count("ch1"))

	// Advance past window
	now = now.Add(61 * time.Second)
	assert.Equal(t, 0, rl.Count("ch1"))
}

func TestBuildSummary_FewMessages(t *testing.T) {
	rl := NewRateLimiter(3600, 1, 3)

	rl.Allow("ch1", newTestMessage("allowed"))
	rl.Allow("ch1", newTestMessage("p1"))
	rl.Allow("ch1", newTestMessage("p2"))
	rl.Allow("ch1", newTestMessage("p3"))

	// This should trigger summary (4 pending > maxPending of 3)
	r := rl.Allow("ch1", newTestMessage("p4"))
	assert.False(t, r.Allowed)
	assert.Contains(t, r.Summary, "p1")
	assert.Contains(t, r.Summary, "p2")
	assert.Contains(t, r.Summary, "p3")
	assert.Contains(t, r.Summary, "p4")
	assert.Contains(t, r.Summary, "4 条暂存消息")
}

func TestBuildSummary_ManyMessages_Truncated(t *testing.T) {
	rl := NewRateLimiter(3600, 1, 12)

	rl.Allow("ch1", newTestMessage("allowed"))

	// Queue 12 messages
	for i := 0; i < 12; i++ {
		rl.Allow("ch1", newTestMessage(fmt.Sprintf("msg-%d", i)))
	}

	// 13th triggers summary
	r := rl.Allow("ch1", newTestMessage("overflow"))
	assert.NotEmpty(t, r.Summary)
	assert.Contains(t, r.Summary, "13 条暂存消息")
	assert.Contains(t, r.Summary, "其他")
}

func TestFlush_ResetsWindowStart(t *testing.T) {
	rl := NewRateLimiter(3600, 2, 100)

	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	rl.nowFunc = func() time.Time { return now }

	// Fill limit and queue
	rl.Allow("ch1", newTestMessage("msg-1"))
	rl.Allow("ch1", newTestMessage("msg-2"))
	rl.Allow("ch1", newTestMessage("msg-3"))

	// Advance time slightly (still within window)
	now = now.Add(10 * time.Second)

	// Flush resets the window
	flushed := rl.Flush("ch1")
	require.Len(t, flushed, 1)

	// Should be able to push again
	r := rl.Allow("ch1", newTestMessage("msg-4"))
	assert.True(t, r.Allowed)
}
