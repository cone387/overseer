package aggregator

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/overseer/overseer/internal/model"
)

// RateLimiter tracks push counts per channel within a configurable time window
// and queues messages when the rate limit is reached.
type RateLimiter struct {
	rateWindow int // window duration in seconds
	rateLimit  int // max pushes per window
	maxPending int // max pending messages before summary

	mu       sync.Mutex
	windows  map[string]*channelWindow
	nowFunc  func() time.Time // injectable clock for testing
}

// channelWindow tracks the push count and pending messages for a single channel.
type channelWindow struct {
	count       int
	windowStart time.Time
	pending     []*model.Message
}

// RateLimitResult describes the outcome of a rate limit check.
type RateLimitResult struct {
	Allowed bool
	// Summary is non-empty when pending exceeded max and a summary was generated.
	Summary  string
	// Flushed contains messages that should be sent (from flush or summary context).
	Flushed  []*model.Message
}

// NewRateLimiter creates a RateLimiter with the given configuration.
// rateWindow is in seconds (default 3600), rateLimit is max pushes per window (default 30),
// maxPending is the max queued messages before summary (default 100).
func NewRateLimiter(rateWindow, rateLimit, maxPending int) *RateLimiter {
	if rateWindow <= 0 {
		rateWindow = 3600
	}
	if rateLimit <= 0 {
		rateLimit = 30
	}
	if maxPending <= 0 {
		maxPending = 100
	}
	return &RateLimiter{
		rateWindow: rateWindow,
		rateLimit:  rateLimit,
		maxPending: maxPending,
		windows:    make(map[string]*channelWindow),
		nowFunc:    time.Now,
	}
}

// Allow checks whether a message can be pushed for the given channel.
// If the rate limit has been reached, the message is queued.
// If pending exceeds maxPending after queuing, a summary is generated and the queue is cleared.
// Returns a RateLimitResult indicating whether the message is allowed through.
func (rl *RateLimiter) Allow(channel string, msg *model.Message) RateLimitResult {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := rl.nowFunc()
	w := rl.getOrCreateWindow(channel, now)

	// Check if we're still within the current window
	elapsed := now.Sub(w.windowStart)
	if elapsed >= time.Duration(rl.rateWindow)*time.Second {
		// Window has expired, reset
		w.count = 0
		w.windowStart = now
	}

	if w.count < rl.rateLimit {
		// Under the limit, allow the push
		w.count++
		return RateLimitResult{Allowed: true}
	}

	// Rate limit reached, queue the message
	w.pending = append(w.pending, msg)

	// Check if pending exceeds max
	if len(w.pending) > rl.maxPending {
		summary := rl.buildSummary(channel, w.pending)
		w.pending = nil
		return RateLimitResult{
			Allowed: false,
			Summary: summary,
		}
	}

	return RateLimitResult{Allowed: false}
}

// Flush returns all pending messages for a channel and resets the window.
// This should be called when a new time window starts.
// Messages are returned in receive order (the order they were queued).
func (rl *RateLimiter) Flush(channel string) []*model.Message {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	w, ok := rl.windows[channel]
	if !ok || len(w.pending) == 0 {
		return nil
	}

	msgs := w.pending
	w.pending = nil
	w.count = 0
	w.windowStart = rl.nowFunc()
	return msgs
}

// PendingCount returns the number of pending messages for a channel.
func (rl *RateLimiter) PendingCount(channel string) int {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	w, ok := rl.windows[channel]
	if !ok {
		return 0
	}
	return len(w.pending)
}

// Count returns the current push count for a channel in the active window.
func (rl *RateLimiter) Count(channel string) int {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := rl.nowFunc()
	w, ok := rl.windows[channel]
	if !ok {
		return 0
	}

	elapsed := now.Sub(w.windowStart)
	if elapsed >= time.Duration(rl.rateWindow)*time.Second {
		return 0
	}
	return w.count
}

func (rl *RateLimiter) getOrCreateWindow(channel string, now time.Time) *channelWindow {
	w, ok := rl.windows[channel]
	if !ok {
		w = &channelWindow{
			windowStart: now,
		}
		rl.windows[channel] = w
	}
	return w
}

func (rl *RateLimiter) buildSummary(channel string, msgs []*model.Message) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("频率限制摘要 [%s]: 共 %d 条暂存消息\n", channel, len(msgs)))
	for i, m := range msgs {
		if i >= 10 {
			sb.WriteString(fmt.Sprintf("... 及其他 %d 条消息\n", len(msgs)-10))
			break
		}
		sb.WriteString(fmt.Sprintf("- %s\n", m.Title))
	}
	return sb.String()
}
