package aggregator

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/overseer/overseer/internal/model"
)

// BatchResult holds the result of a batch merge operation.
type BatchResult struct {
	Messages    []*model.Message // All accumulated messages in the batch
	TotalCount  int              // Total number of messages in the batch
	WindowStart time.Time        // Start time of the batch window
	WindowEnd   time.Time        // End time of the batch window
	Titles      []string         // List of message titles
}

// Summary returns a formatted summary string for the batch.
func (br *BatchResult) Summary() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📦 消息合并摘要 (%d 条)\n", br.TotalCount))
	sb.WriteString(fmt.Sprintf("时间窗口: %s ~ %s\n",
		br.WindowStart.Format("15:04:05"),
		br.WindowEnd.Format("15:04:05"),
	))
	sb.WriteString("消息列表:\n")
	for i, title := range br.Titles {
		sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, title))
	}
	return sb.String()
}

// channelBatch tracks accumulated messages for a single channel.
type channelBatch struct {
	messages    []*model.Message
	windowStart time.Time
}

// Batcher tracks message counts per channel within a configurable time window
// and merges them into a summary push when the threshold is exceeded.
type Batcher struct {
	threshold  int           // Number of messages before triggering a batch
	window     time.Duration // Time window for accumulating messages
	batches    map[string]*channelBatch
	mu         sync.Mutex
}

// NewBatcher creates a new Batcher with the given threshold and window duration.
// If threshold <= 0, defaults to 10. If window <= 0, defaults to 5 minutes.
func NewBatcher(threshold int, window time.Duration) *Batcher {
	if threshold <= 0 {
		threshold = 10
	}
	if window <= 0 {
		window = 5 * time.Minute
	}
	return &Batcher{
		threshold: threshold,
		window:    window,
		batches:   make(map[string]*channelBatch),
	}
}

// Add adds a message to the batcher for the given channel.
// It returns a non-nil BatchResult when the threshold is exceeded,
// containing all accumulated messages. After returning a batch result,
// the counter for that channel is reset.
func (b *Batcher) Add(channel string, msg *model.Message) *BatchResult {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := msg.ReceivedAt
	batch, exists := b.batches[channel]

	// If no existing batch or the window has expired, start a new one.
	if !exists || now.Sub(batch.windowStart) > b.window {
		b.batches[channel] = &channelBatch{
			messages:    []*model.Message{msg},
			windowStart: now,
		}
		return nil
	}

	// Add message to the existing batch.
	batch.messages = append(batch.messages, msg)

	// Check if threshold is exceeded.
	if len(batch.messages) > b.threshold {
		result := b.buildResult(batch, now)
		// Reset the batch for this channel.
		delete(b.batches, channel)
		return result
	}

	return nil
}

// Flush forces a batch result for the given channel regardless of threshold.
// Returns nil if no messages are accumulated for the channel.
func (b *Batcher) Flush(channel string) *BatchResult {
	b.mu.Lock()
	defer b.mu.Unlock()

	batch, exists := b.batches[channel]
	if !exists || len(batch.messages) == 0 {
		return nil
	}

	now := time.Now()
	result := b.buildResult(batch, now)
	delete(b.batches, channel)
	return result
}

// Count returns the current number of accumulated messages for a channel.
func (b *Batcher) Count(channel string) int {
	b.mu.Lock()
	defer b.mu.Unlock()

	batch, exists := b.batches[channel]
	if !exists {
		return 0
	}
	return len(batch.messages)
}

// Threshold returns the configured batch threshold.
func (b *Batcher) Threshold() int {
	return b.threshold
}

// Window returns the configured time window.
func (b *Batcher) Window() time.Duration {
	return b.window
}

// buildResult constructs a BatchResult from the accumulated messages.
func (b *Batcher) buildResult(batch *channelBatch, windowEnd time.Time) *BatchResult {
	titles := make([]string, len(batch.messages))
	for i, m := range batch.messages {
		titles[i] = m.Title
	}

	return &BatchResult{
		Messages:    batch.messages,
		TotalCount:  len(batch.messages),
		WindowStart: batch.windowStart,
		WindowEnd:   windowEnd,
		Titles:      titles,
	}
}
