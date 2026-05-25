package aggregator

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// dedupEntry tracks a deduplicated message within a time window.
type dedupEntry struct {
	FirstSeen time.Time
	Count     int
}

// Deduper tracks messages by channel + hash(title+body) and suppresses
// duplicates within a configurable time window.
type Deduper struct {
	window  time.Duration
	entries map[string]*dedupEntry
	mu      sync.Mutex
	nowFunc func() time.Time // for testing
}

// DedupeResult represents the outcome of a deduplication check.
type DedupeResult struct {
	// IsDuplicate is true if the message was already seen within the window.
	IsDuplicate bool
	// Count is the total number of times this message has been seen in the
	// current window (including the first occurrence).
	Count int
}

// NewDeduper creates a new Deduper with the given time window in seconds.
// If windowSeconds <= 0, defaults to 300 (5 minutes).
func NewDeduper(windowSeconds int) *Deduper {
	if windowSeconds <= 0 {
		windowSeconds = 300
	}
	return &Deduper{
		window:  time.Duration(windowSeconds) * time.Second,
		entries: make(map[string]*dedupEntry),
		nowFunc: time.Now,
	}
}

// Check determines whether a message (identified by channel, title, body) is a
// duplicate within the configured time window.
// Returns a DedupeResult indicating whether it's a duplicate and the current count.
func (d *Deduper) Check(channel, title, body string) DedupeResult {
	key := d.buildKey(channel, title, body)
	now := d.nowFunc()

	d.mu.Lock()
	defer d.mu.Unlock()

	entry, exists := d.entries[key]
	if exists && now.Sub(entry.FirstSeen) < d.window {
		// Still within the window — this is a duplicate.
		entry.Count++
		return DedupeResult{IsDuplicate: true, Count: entry.Count}
	}

	// Either first time or window expired — start a new window.
	d.entries[key] = &dedupEntry{
		FirstSeen: now,
		Count:     1,
	}
	return DedupeResult{IsDuplicate: false, Count: 1}
}

// Cleanup removes expired entries from the deduper.
func (d *Deduper) Cleanup() {
	now := d.nowFunc()

	d.mu.Lock()
	defer d.mu.Unlock()

	for key, entry := range d.entries {
		if now.Sub(entry.FirstSeen) >= d.window {
			delete(d.entries, key)
		}
	}
}

// Len returns the number of tracked entries (for testing/monitoring).
func (d *Deduper) Len() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.entries)
}

// buildKey creates a unique key from channel + sha256(title+body).
func (d *Deduper) buildKey(channel, title, body string) string {
	h := sha256.New()
	h.Write([]byte(title))
	h.Write([]byte{0}) // separator to avoid collisions like ("ab","c") vs ("a","bc")
	h.Write([]byte(body))
	hash := hex.EncodeToString(h.Sum(nil))
	return channel + ":" + hash
}
