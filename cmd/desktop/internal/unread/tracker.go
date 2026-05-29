package unread

import (
	"log"
	"sync"

	"github.com/overseer/overseer/cmd/desktop/internal/cache"
)

// OnUpdateFunc is called when the unread count changes.
type OnUpdateFunc func(unreadCount int)

// Tracker coordinates between cache state and tray icon.
type Tracker struct {
	cache    *cache.Cache
	onUpdate OnUpdateFunc
	mu       sync.Mutex
}

// New creates a new Tracker with the given cache and update callback.
func New(c *cache.Cache, onUpdate OnUpdateFunc) *Tracker {
	return &Tracker{
		cache:    c,
		onUpdate: onUpdate,
	}
}

// OnPush is called when a new push notification arrives.
// The notification should already be stored in the cache as unread.
func (t *Tracker) OnPush(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.notifyUpdate()
}

// OnRead marks a specific notification as read.
func (t *Tracker) OnRead(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.cache.MarkRead(id); err != nil {
		log.Printf("[unread] mark read %s: %v", id, err)
	}
	t.notifyUpdate()
}

// OnReadAll marks all notifications as read.
func (t *Tracker) OnReadAll() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.cache.MarkAllRead(); err != nil {
		log.Printf("[unread] mark all read: %v", err)
	}
	t.notifyUpdate()
}

// OnAckSync handles an ack_sync event from another device.
func (t *Tracker) OnAckSync(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.cache.MarkAckSynced(id); err != nil {
		log.Printf("[unread] ack sync %s: %v", id, err)
	}
	t.notifyUpdate()
}

// OnExpired handles a message expiry.
func (t *Tracker) OnExpired(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.cache.MarkRead(id); err != nil {
		log.Printf("[unread] expire %s: %v", id, err)
	}
	t.notifyUpdate()
}

// UnreadCount returns the current unread count.
func (t *Tracker) UnreadCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()

	count, err := t.cache.UnreadCount()
	if err != nil {
		log.Printf("[unread] count: %v", err)
		return 0
	}
	return count
}

// ShouldFlash returns true if the tray icon should be flashing.
func (t *Tracker) ShouldFlash() bool {
	return t.UnreadCount() > 0
}

// notifyUpdate queries the cache for the current unread count and calls the onUpdate callback.
// Must be called with t.mu held.
func (t *Tracker) notifyUpdate() {
	count, err := t.cache.UnreadCount()
	if err != nil {
		log.Printf("[unread] count: %v", err)
		count = 0
	}
	if t.onUpdate != nil {
		t.onUpdate(count)
	}
}
