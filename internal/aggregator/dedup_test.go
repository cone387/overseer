package aggregator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewDeduper_DefaultWindow(t *testing.T) {
	d := NewDeduper(0)
	assert.Equal(t, 300*time.Second, d.window)
}

func TestNewDeduper_NegativeWindow(t *testing.T) {
	d := NewDeduper(-10)
	assert.Equal(t, 300*time.Second, d.window)
}

func TestNewDeduper_CustomWindow(t *testing.T) {
	d := NewDeduper(60)
	assert.Equal(t, 60*time.Second, d.window)
}

func TestDeduper_FirstMessagePassesThrough(t *testing.T) {
	d := NewDeduper(300)
	result := d.Check("alerts", "Server Down", "host-1 is unreachable")

	assert.False(t, result.IsDuplicate)
	assert.Equal(t, 1, result.Count)
}

func TestDeduper_DuplicateWithinWindow(t *testing.T) {
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	d := NewDeduper(300)
	d.nowFunc = func() time.Time { return now }

	// First message passes through.
	r1 := d.Check("alerts", "Server Down", "host-1 is unreachable")
	assert.False(t, r1.IsDuplicate)
	assert.Equal(t, 1, r1.Count)

	// Same message 1 minute later — duplicate.
	d.nowFunc = func() time.Time { return now.Add(1 * time.Minute) }
	r2 := d.Check("alerts", "Server Down", "host-1 is unreachable")
	assert.True(t, r2.IsDuplicate)
	assert.Equal(t, 2, r2.Count)

	// Third duplicate.
	d.nowFunc = func() time.Time { return now.Add(2 * time.Minute) }
	r3 := d.Check("alerts", "Server Down", "host-1 is unreachable")
	assert.True(t, r3.IsDuplicate)
	assert.Equal(t, 3, r3.Count)
}

func TestDeduper_DifferentChannelNotDuplicate(t *testing.T) {
	d := NewDeduper(300)

	r1 := d.Check("alerts", "Server Down", "host-1 is unreachable")
	assert.False(t, r1.IsDuplicate)

	// Same title+body but different channel — not a duplicate.
	r2 := d.Check("monitoring", "Server Down", "host-1 is unreachable")
	assert.False(t, r2.IsDuplicate)
}

func TestDeduper_DifferentContentNotDuplicate(t *testing.T) {
	d := NewDeduper(300)

	r1 := d.Check("alerts", "Server Down", "host-1 is unreachable")
	assert.False(t, r1.IsDuplicate)

	// Same channel but different body.
	r2 := d.Check("alerts", "Server Down", "host-2 is unreachable")
	assert.False(t, r2.IsDuplicate)

	// Same channel but different title.
	r3 := d.Check("alerts", "Disk Full", "host-1 is unreachable")
	assert.False(t, r3.IsDuplicate)
}

func TestDeduper_WindowExpiry(t *testing.T) {
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	d := NewDeduper(300) // 5 minutes
	d.nowFunc = func() time.Time { return now }

	r1 := d.Check("alerts", "Server Down", "host-1")
	assert.False(t, r1.IsDuplicate)

	// 5 minutes later — window expired, message passes through again.
	d.nowFunc = func() time.Time { return now.Add(5 * time.Minute) }
	r2 := d.Check("alerts", "Server Down", "host-1")
	assert.False(t, r2.IsDuplicate)
	assert.Equal(t, 1, r2.Count)
}

func TestDeduper_WindowExpiryJustBefore(t *testing.T) {
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	d := NewDeduper(300)
	d.nowFunc = func() time.Time { return now }

	d.Check("alerts", "Server Down", "host-1")

	// 4m59s later — still within window.
	d.nowFunc = func() time.Time { return now.Add(4*time.Minute + 59*time.Second) }
	r := d.Check("alerts", "Server Down", "host-1")
	assert.True(t, r.IsDuplicate)
}

func TestDeduper_Cleanup(t *testing.T) {
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	d := NewDeduper(300)
	d.nowFunc = func() time.Time { return now }

	d.Check("alerts", "msg1", "body1")
	d.Check("alerts", "msg2", "body2")
	assert.Equal(t, 2, d.Len())

	// Advance time past the window.
	d.nowFunc = func() time.Time { return now.Add(6 * time.Minute) }
	d.Cleanup()
	assert.Equal(t, 0, d.Len())
}

func TestDeduper_CleanupPartial(t *testing.T) {
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	d := NewDeduper(300)
	d.nowFunc = func() time.Time { return now }

	d.Check("alerts", "msg1", "body1")

	// Add second message 3 minutes later.
	d.nowFunc = func() time.Time { return now.Add(3 * time.Minute) }
	d.Check("alerts", "msg2", "body2")

	// At 5 minutes, first entry expired but second still valid.
	d.nowFunc = func() time.Time { return now.Add(5 * time.Minute) }
	d.Cleanup()
	assert.Equal(t, 1, d.Len())
}

func TestDeduper_ConcurrentAccess(t *testing.T) {
	d := NewDeduper(300)
	done := make(chan struct{})

	// Run concurrent checks to verify no race conditions.
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				d.Check("ch", "title", "body")
			}
			done <- struct{}{}
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// All 1000 calls after the first should be duplicates.
	// The entry should exist with count = 1000.
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, entry := range d.entries {
		assert.Equal(t, 1000, entry.Count)
	}
}

func TestDeduper_EmptyFields(t *testing.T) {
	d := NewDeduper(300)

	// Empty title and body should still work.
	r1 := d.Check("ch", "", "")
	assert.False(t, r1.IsDuplicate)

	r2 := d.Check("ch", "", "")
	assert.True(t, r2.IsDuplicate)
}

func TestDeduper_HashCollisionAvoidance(t *testing.T) {
	d := NewDeduper(300)

	// Ensure "ab" + "c" is different from "a" + "bc" due to separator.
	r1 := d.Check("ch", "ab", "c")
	assert.False(t, r1.IsDuplicate)

	r2 := d.Check("ch", "a", "bc")
	assert.False(t, r2.IsDuplicate)
}
