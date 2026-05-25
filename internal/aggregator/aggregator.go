package aggregator

import (
	"time"

	"github.com/overseer/overseer/internal/config"
	"github.com/overseer/overseer/internal/model"
)

// Action represents the aggregation decision for a message.
type Action int

const (
	// ActionPass means the message should be pushed immediately.
	ActionPass Action = iota
	// ActionDedupe means the message is a duplicate and was suppressed.
	ActionDedupe
	// ActionBatch means the message was merged into a batch summary.
	ActionBatch
	// ActionRateLimit means the message was rate-limited and queued.
	ActionRateLimit
)

// AggregateResult describes the outcome of processing a message through the aggregator.
type AggregateResult struct {
	Action    Action
	Messages  []*model.Message
	Summary   string
	DupeCount int
}

// Aggregator combines deduplication, batching, and rate limiting into a single
// processing pipeline. Messages with critical priority bypass all rules.
type Aggregator struct {
	config      config.AggregatorConfig
	deduper     *Deduper
	batcher     *Batcher
	rateLimiter *RateLimiter
}

// NewAggregator creates an Aggregator with the given configuration.
func NewAggregator(cfg config.AggregatorConfig) *Aggregator {
	batchWindow := time.Duration(cfg.DedupeWindow) * time.Second
	if batchWindow <= 0 {
		batchWindow = 5 * time.Minute
	}

	return &Aggregator{
		config:      cfg,
		deduper:     NewDeduper(cfg.DedupeWindow),
		batcher:     NewBatcher(cfg.BatchThreshold, batchWindow),
		rateLimiter: NewRateLimiter(cfg.RateWindow, cfg.RateLimit, cfg.MaxPending),
	}
}

// Process evaluates a message against all aggregation rules and returns the result.
// The level parameter is the channel's priority level (e.g., "critical", "active").
//
// Processing order:
// 1. Critical priority messages bypass all aggregation rules (ActionPass).
// 2. Deduplication check — duplicates are suppressed (ActionDedupe).
// 3. Batch check — if threshold exceeded, messages are merged (ActionBatch).
// 4. Rate limit check — if limit reached, message is queued (ActionRateLimit).
// 5. Otherwise the message passes through (ActionPass).
func (a *Aggregator) Process(msg *model.Message, level string) AggregateResult {
	// 1. Critical priority bypasses all aggregation.
	if level == "critical" {
		return AggregateResult{
			Action:   ActionPass,
			Messages: []*model.Message{msg},
		}
	}

	// 2. Deduplication check.
	dedupResult := a.deduper.Check(msg.Channel, msg.Title, msg.Body)
	if dedupResult.IsDuplicate {
		return AggregateResult{
			Action:    ActionDedupe,
			DupeCount: dedupResult.Count,
		}
	}

	// 3. Batch check.
	batchResult := a.batcher.Add(msg.Channel, msg)
	if batchResult != nil {
		return AggregateResult{
			Action:   ActionBatch,
			Messages: batchResult.Messages,
			Summary:  batchResult.Summary(),
		}
	}

	// 4. Rate limit check.
	rlResult := a.rateLimiter.Allow(msg.Channel, msg)
	if !rlResult.Allowed {
		result := AggregateResult{
			Action: ActionRateLimit,
		}
		if rlResult.Summary != "" {
			result.Summary = rlResult.Summary
		}
		return result
	}

	// 5. Message passes through.
	return AggregateResult{
		Action:   ActionPass,
		Messages: []*model.Message{msg},
	}
}
