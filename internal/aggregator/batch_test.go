package aggregator

import (
	"testing"
	"time"

	"github.com/overseer/overseer/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeMsg(title string, receivedAt time.Time) *model.Message {
	return &model.Message{
		ID:         title,
		Title:      title,
		Body:       "body of " + title,
		ReceivedAt: receivedAt,
	}
}

func TestNewBatcher_Defaults(t *testing.T) {
	b := NewBatcher(0, 0)
	assert.Equal(t, 10, b.Threshold())
	assert.Equal(t, 5*time.Minute, b.Window())
}

func TestNewBatcher_CustomValues(t *testing.T) {
	b := NewBatcher(5, 2*time.Minute)
	assert.Equal(t, 5, b.Threshold())
	assert.Equal(t, 2*time.Minute, b.Window())
}

func TestBatcher_BelowThreshold_ReturnsNil(t *testing.T) {
	b := NewBatcher(3, 5*time.Minute)
	now := time.Now()

	for i := 0; i < 3; i++ {
		msg := makeMsg("msg"+string(rune('A'+i)), now.Add(time.Duration(i)*time.Second))
		result := b.Add("ch1", msg)
		assert.Nil(t, result, "should not batch when at or below threshold")
	}

	assert.Equal(t, 3, b.Count("ch1"))
}

func TestBatcher_ExceedsThreshold_ReturnsBatchResult(t *testing.T) {
	threshold := 3
	b := NewBatcher(threshold, 5*time.Minute)
	now := time.Now()

	// Add threshold messages (no batch yet)
	for i := 0; i < threshold; i++ {
		msg := makeMsg("msg"+string(rune('A'+i)), now.Add(time.Duration(i)*time.Second))
		result := b.Add("ch1", msg)
		assert.Nil(t, result)
	}

	// The next message exceeds the threshold and triggers batching
	triggerMsg := makeMsg("trigger", now.Add(time.Duration(threshold)*time.Second))
	result := b.Add("ch1", triggerMsg)

	require.NotNil(t, result)
	assert.Equal(t, threshold+1, result.TotalCount)
	assert.Equal(t, threshold+1, len(result.Messages))
	assert.Equal(t, threshold+1, len(result.Titles))
	assert.Equal(t, now, result.WindowStart)
	assert.Equal(t, triggerMsg.ReceivedAt, result.WindowEnd)

	// Verify titles are in order
	assert.Equal(t, "msgA", result.Titles[0])
	assert.Equal(t, "msgB", result.Titles[1])
	assert.Equal(t, "msgC", result.Titles[2])
	assert.Equal(t, "trigger", result.Titles[3])
}

func TestBatcher_ResetsAfterBatch(t *testing.T) {
	b := NewBatcher(2, 5*time.Minute)
	now := time.Now()

	// Fill and trigger batch
	b.Add("ch1", makeMsg("a", now))
	b.Add("ch1", makeMsg("b", now.Add(1*time.Second)))
	result := b.Add("ch1", makeMsg("c", now.Add(2*time.Second)))
	require.NotNil(t, result)

	// After batch, counter should be reset
	assert.Equal(t, 0, b.Count("ch1"))

	// New messages start fresh
	result = b.Add("ch1", makeMsg("d", now.Add(3*time.Second)))
	assert.Nil(t, result)
	assert.Equal(t, 1, b.Count("ch1"))
}

func TestBatcher_WindowExpiry_StartsNewBatch(t *testing.T) {
	b := NewBatcher(5, 1*time.Minute)
	now := time.Now()

	// Add a message
	b.Add("ch1", makeMsg("early", now))
	assert.Equal(t, 1, b.Count("ch1"))

	// Add a message after the window expires
	laterMsg := makeMsg("late", now.Add(2*time.Minute))
	result := b.Add("ch1", laterMsg)
	assert.Nil(t, result)

	// The old batch was discarded, new batch starts with just the late message
	assert.Equal(t, 1, b.Count("ch1"))
}

func TestBatcher_MultipleChannels_Independent(t *testing.T) {
	b := NewBatcher(2, 5*time.Minute)
	now := time.Now()

	// Add messages to different channels
	b.Add("ch1", makeMsg("ch1-a", now))
	b.Add("ch2", makeMsg("ch2-a", now))
	b.Add("ch1", makeMsg("ch1-b", now.Add(1*time.Second)))
	b.Add("ch2", makeMsg("ch2-b", now.Add(1*time.Second)))

	assert.Equal(t, 2, b.Count("ch1"))
	assert.Equal(t, 2, b.Count("ch2"))

	// Trigger batch on ch1 only
	result := b.Add("ch1", makeMsg("ch1-c", now.Add(2*time.Second)))
	require.NotNil(t, result)
	assert.Equal(t, 3, result.TotalCount)

	// ch2 should be unaffected
	assert.Equal(t, 0, b.Count("ch1"))
	assert.Equal(t, 2, b.Count("ch2"))
}

func TestBatcher_Flush_ReturnsAccumulated(t *testing.T) {
	b := NewBatcher(10, 5*time.Minute)
	now := time.Now()

	b.Add("ch1", makeMsg("a", now))
	b.Add("ch1", makeMsg("b", now.Add(1*time.Second)))

	result := b.Flush("ch1")
	require.NotNil(t, result)
	assert.Equal(t, 2, result.TotalCount)
	assert.Equal(t, []string{"a", "b"}, result.Titles)

	// After flush, channel is empty
	assert.Equal(t, 0, b.Count("ch1"))
}

func TestBatcher_Flush_EmptyChannel_ReturnsNil(t *testing.T) {
	b := NewBatcher(10, 5*time.Minute)

	result := b.Flush("nonexistent")
	assert.Nil(t, result)
}

func TestBatcher_Count_NonexistentChannel(t *testing.T) {
	b := NewBatcher(10, 5*time.Minute)
	assert.Equal(t, 0, b.Count("nonexistent"))
}

func TestBatchResult_Summary(t *testing.T) {
	now := time.Now()
	result := &BatchResult{
		TotalCount:  3,
		WindowStart: now,
		WindowEnd:   now.Add(2 * time.Minute),
		Titles:      []string{"Alert 1", "Alert 2", "Alert 3"},
	}

	summary := result.Summary()
	assert.Contains(t, summary, "3 条")
	assert.Contains(t, summary, now.Format("15:04:05"))
	assert.Contains(t, summary, now.Add(2*time.Minute).Format("15:04:05"))
	assert.Contains(t, summary, "1. Alert 1")
	assert.Contains(t, summary, "2. Alert 2")
	assert.Contains(t, summary, "3. Alert 3")
}
