package aggregator

import (
	"testing"
	"time"

	"github.com/overseer/overseer/internal/config"
	"github.com/overseer/overseer/internal/model"
	"github.com/stretchr/testify/assert"
)

func newTestAggregator() *Aggregator {
	cfg := config.AggregatorConfig{
		DedupeWindow:   300,
		BatchThreshold: 3,
		RateWindow:     3600,
		RateLimit:      5,
		MaxPending:     100,
	}
	return NewAggregator(cfg)
}

func newMsg(channel, title, body string) *model.Message {
	return &model.Message{
		ID:         "msg-" + title,
		Source:     "test",
		Channel:    channel,
		Title:      title,
		Body:       body,
		ReceivedAt: time.Now(),
	}
}

func TestProcess_CriticalBypassesAllRules(t *testing.T) {
	agg := newTestAggregator()

	// Send the same message multiple times with critical level — all should pass.
	for i := 0; i < 10; i++ {
		msg := newMsg("urgent", "alert", "server down")
		result := agg.Process(msg, "critical")
		assert.Equal(t, ActionPass, result.Action, "critical messages should always pass")
		assert.Equal(t, []*model.Message{msg}, result.Messages)
	}
}

func TestProcess_CriticalBypassesDedup(t *testing.T) {
	agg := newTestAggregator()

	msg1 := newMsg("urgent", "alert", "server down")
	msg2 := newMsg("urgent", "alert", "server down")

	// First message passes normally.
	r1 := agg.Process(msg1, "active")
	assert.Equal(t, ActionPass, r1.Action)

	// Second identical message with critical level should still pass.
	r2 := agg.Process(msg2, "critical")
	assert.Equal(t, ActionPass, r2.Action)
	assert.Equal(t, []*model.Message{msg2}, r2.Messages)
}

func TestProcess_CriticalBypassesRateLimit(t *testing.T) {
	cfg := config.AggregatorConfig{
		DedupeWindow:   300,
		BatchThreshold: 100,
		RateWindow:     3600,
		RateLimit:      2,
		MaxPending:     100,
	}
	agg := NewAggregator(cfg)

	// Exhaust rate limit with non-critical messages.
	for i := 0; i < 2; i++ {
		msg := newMsg("ch", "title"+string(rune('A'+i)), "body"+string(rune('A'+i)))
		agg.Process(msg, "active")
	}

	// Next non-critical message should be rate-limited.
	msg := newMsg("ch", "titleX", "bodyX")
	r := agg.Process(msg, "active")
	assert.Equal(t, ActionRateLimit, r.Action)

	// But critical message on same channel should pass.
	critMsg := newMsg("ch", "titleCrit", "bodyCrit")
	rCrit := agg.Process(critMsg, "critical")
	assert.Equal(t, ActionPass, rCrit.Action)
}

func TestProcess_DedupSuppressesDuplicates(t *testing.T) {
	agg := newTestAggregator()

	msg1 := newMsg("github", "PR merged", "PR #42 merged")
	msg2 := newMsg("github", "PR merged", "PR #42 merged")

	r1 := agg.Process(msg1, "active")
	assert.Equal(t, ActionPass, r1.Action)

	r2 := agg.Process(msg2, "active")
	assert.Equal(t, ActionDedupe, r2.Action)
	assert.Equal(t, 2, r2.DupeCount)
}

func TestProcess_DedupCountIncreases(t *testing.T) {
	agg := newTestAggregator()

	msg := newMsg("monitor", "CPU high", "CPU > 90%")

	r1 := agg.Process(msg, "active")
	assert.Equal(t, ActionPass, r1.Action)

	for i := 2; i <= 5; i++ {
		dup := newMsg("monitor", "CPU high", "CPU > 90%")
		r := agg.Process(dup, "active")
		assert.Equal(t, ActionDedupe, r.Action)
		assert.Equal(t, i, r.DupeCount)
	}
}

func TestProcess_BatchMergesWhenThresholdExceeded(t *testing.T) {
	cfg := config.AggregatorConfig{
		DedupeWindow:   300,
		BatchThreshold: 3,
		RateWindow:     3600,
		RateLimit:      100,
		MaxPending:     100,
	}
	agg := NewAggregator(cfg)

	// Send threshold+1 different messages to trigger batch.
	var results []AggregateResult
	for i := 0; i < 4; i++ {
		msg := newMsg("github", "event"+string(rune('A'+i)), "body"+string(rune('A'+i)))
		r := agg.Process(msg, "active")
		results = append(results, r)
	}

	// First 3 should pass, 4th triggers batch.
	for i := 0; i < 3; i++ {
		assert.Equal(t, ActionPass, results[i].Action, "message %d should pass", i)
	}
	assert.Equal(t, ActionBatch, results[3].Action)
	assert.NotEmpty(t, results[3].Summary)
	assert.Equal(t, 4, len(results[3].Messages))
}

func TestProcess_RateLimitQueuesMessages(t *testing.T) {
	cfg := config.AggregatorConfig{
		DedupeWindow:   300,
		BatchThreshold: 100,
		RateWindow:     3600,
		RateLimit:      2,
		MaxPending:     100,
	}
	agg := NewAggregator(cfg)

	// Send 2 messages to exhaust rate limit.
	for i := 0; i < 2; i++ {
		msg := newMsg("monitor", "alert"+string(rune('A'+i)), "body"+string(rune('A'+i)))
		r := agg.Process(msg, "active")
		assert.Equal(t, ActionPass, r.Action)
	}

	// 3rd message should be rate-limited.
	msg := newMsg("monitor", "alertC", "bodyC")
	r := agg.Process(msg, "active")
	assert.Equal(t, ActionRateLimit, r.Action)
}

func TestProcess_PassForNormalMessage(t *testing.T) {
	agg := newTestAggregator()

	msg := newMsg("default", "hello", "world")
	r := agg.Process(msg, "active")

	assert.Equal(t, ActionPass, r.Action)
	assert.Equal(t, []*model.Message{msg}, r.Messages)
	assert.Empty(t, r.Summary)
	assert.Equal(t, 0, r.DupeCount)
}

func TestProcess_DifferentChannelsIndependent(t *testing.T) {
	agg := newTestAggregator()

	msg1 := newMsg("ch1", "same title", "same body")
	msg2 := newMsg("ch2", "same title", "same body")

	r1 := agg.Process(msg1, "active")
	r2 := agg.Process(msg2, "active")

	// Same content but different channels — both should pass.
	assert.Equal(t, ActionPass, r1.Action)
	assert.Equal(t, ActionPass, r2.Action)
}

func TestNewAggregator_DefaultConfig(t *testing.T) {
	cfg := config.AggregatorConfig{} // all zeros
	agg := NewAggregator(cfg)

	assert.NotNil(t, agg.deduper)
	assert.NotNil(t, agg.batcher)
	assert.NotNil(t, agg.rateLimiter)
}
