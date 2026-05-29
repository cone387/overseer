package grouper

import (
	"fmt"
	"sync"
	"time"
)

// PushEvent mirrors the wsclient.PushEvent fields needed for grouping.
type PushEvent struct {
	ID      string
	Source  string
	Channel string
	Title   string
	Body    string
	URL     string
	Level   string
}

// ShowFunc is the callback to display a notification.
type ShowFunc func(title, body, url, channel, source, level, msgID string)

// Grouper batches rapid notifications from the same source within a time window.
type Grouper struct {
	window   time.Duration // 30 seconds
	pending  map[string]*pendingGroup
	mu       sync.Mutex
	showFunc ShowFunc
}

type pendingGroup struct {
	source string
	events []PushEvent
	timer  *time.Timer
}

// New creates a new Grouper with a 30-second window.
func New(showFunc ShowFunc) *Grouper {
	return &Grouper{
		window:   30 * time.Second,
		pending:  make(map[string]*pendingGroup),
		showFunc: showFunc,
	}
}

// Ingest processes an incoming push event.
// If another event from the same source arrived within the window, it groups them.
// Otherwise, it shows the notification immediately.
func (g *Grouper) Ingest(event PushEvent) {
	g.mu.Lock()
	defer g.mu.Unlock()

	group, exists := g.pending[event.Source]
	if exists {
		// Add to existing group
		group.events = append(group.events, event)
		// Reset timer
		group.timer.Reset(g.window)
		return
	}

	// First event from this source — start a new group with a timer
	pg := &pendingGroup{
		source: event.Source,
		events: []PushEvent{event},
	}
	pg.timer = time.AfterFunc(g.window, func() {
		g.flush(event.Source)
	})
	g.pending[event.Source] = pg

	// Show the first notification immediately (don't wait for grouping)
	g.showFunc(event.Title, event.Body, event.URL, event.Channel, event.Source, event.Level, event.ID)
}

// flush is called when the grouping window expires.
// If more than 1 event accumulated, show a summary.
func (g *Grouper) flush(source string) {
	g.mu.Lock()
	group, exists := g.pending[source]
	if !exists {
		g.mu.Unlock()
		return
	}
	delete(g.pending, source)
	g.mu.Unlock()

	// If only 1 event, it was already shown immediately
	if len(group.events) <= 1 {
		return
	}

	// Show summary for events 2+ (first was already shown)
	count := len(group.events) - 1 // subtract the first one already shown
	if count > 0 {
		title := fmt.Sprintf("[%s] %d 条新通知", source, count)
		body := group.events[len(group.events)-1].Title // show latest title as body
		g.showFunc(title, body, "", group.events[0].Channel, source, "default", "")
	}
}
