// Package scheduler provides the Do-Not-Disturb (DND) scheduling logic.
// It determines whether messages should be deferred based on configured
// quiet periods and manages a pending queue for deferred messages.
package scheduler

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/overseer/overseer/internal/config"
	"github.com/overseer/overseer/internal/model"
)

// DayMask is a bitmask representing which days of the week a DND period applies.
type DayMask uint8

const (
	DaySunday    DayMask = 1 << iota // 0
	DayMonday                        // 1
	DayTuesday                       // 2
	DayWednesday                     // 3
	DayThursday                      // 4
	DayFriday                        // 5
	DaySaturday                      // 6
)

const (
	DaysEveryday DayMask = DaySunday | DayMonday | DayTuesday | DayWednesday | DayThursday | DayFriday | DaySaturday
	DaysWeekday  DayMask = DayMonday | DayTuesday | DayWednesday | DayThursday | DayFriday
	DaysWeekend  DayMask = DaySaturday | DaySunday
)

// dayMaskForWeekday converts a time.Weekday to the corresponding DayMask bit.
func dayMaskForWeekday(wd time.Weekday) DayMask {
	switch wd {
	case time.Sunday:
		return DaySunday
	case time.Monday:
		return DayMonday
	case time.Tuesday:
		return DayTuesday
	case time.Wednesday:
		return DayWednesday
	case time.Thursday:
		return DayThursday
	case time.Friday:
		return DayFriday
	case time.Saturday:
		return DaySaturday
	default:
		return 0
	}
}

// CompiledDNDPeriod represents a parsed DND period with time offsets from midnight.
type CompiledDNDPeriod struct {
	Start        time.Duration // offset from midnight
	End          time.Duration // offset from midnight
	Days         DayMask
	CrossMidnight bool
}

// DefaultMaxPending is the default maximum number of pending messages.
const DefaultMaxPending = 200

// DNDScheduler manages Do-Not-Disturb periods and deferred message queuing.
type DNDScheduler struct {
	periods    []CompiledDNDPeriod
	pending    []*model.Message
	maxPending int
	mu         sync.Mutex
	nowFunc    func() time.Time // injectable for testing
}

// NewDNDScheduler creates a DNDScheduler from the given DND period configurations.
// maxPending controls the maximum number of deferred messages (0 uses DefaultMaxPending).
func NewDNDScheduler(periods []config.DNDPeriod, maxPending int) (*DNDScheduler, error) {
	if maxPending <= 0 {
		maxPending = DefaultMaxPending
	}

	compiled, err := compilePeriods(periods)
	if err != nil {
		return nil, err
	}

	return &DNDScheduler{
		periods:    compiled,
		pending:    make([]*model.Message, 0),
		maxPending: maxPending,
		nowFunc:    time.Now,
	}, nil
}

// SetNowFunc sets a custom time function for testing.
func (s *DNDScheduler) SetNowFunc(fn func() time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nowFunc = fn
}

// ShouldDefer returns true if the message should be deferred (i.e., the current
// time is within a DND period and the channel level is not urgent/critical).
func (s *DNDScheduler) ShouldDefer(channelLevel string, now time.Time) bool {
	// Urgent and critical messages always pass through
	if isUrgentLevel(channelLevel) {
		return false
	}

	return s.isInDND(now)
}

// isInDND checks whether the given time falls within any active DND period.
func (s *DNDScheduler) isInDND(now time.Time) bool {
	weekday := now.Weekday()
	dayBit := dayMaskForWeekday(weekday)
	offset := timeOfDayOffset(now)

	for _, p := range s.periods {
		if p.CrossMidnight {
			// Cross-midnight: check if we're in the evening portion (today)
			// or the morning portion (started yesterday)
			if p.Days&dayBit != 0 && offset >= p.Start {
				return true
			}
			// Check if we're in the morning portion: the period started yesterday
			yesterdayBit := dayMaskForWeekday(previousWeekday(weekday))
			if p.Days&yesterdayBit != 0 && offset < p.End {
				return true
			}
		} else {
			// Same-day period
			if p.Days&dayBit != 0 && offset >= p.Start && offset < p.End {
				return true
			}
		}
	}
	return false
}

// Enqueue adds a message to the pending queue. If the queue is full (maxPending),
// the oldest message is discarded to make room.
func (s *DNDScheduler) Enqueue(msg *model.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.pending) >= s.maxPending {
		// Discard the oldest message (index 0)
		s.pending = s.pending[1:]
	}
	s.pending = append(s.pending, msg)
}

// FlushPending returns all pending messages sorted by ReceivedAt ascending
// and clears the pending queue.
func (s *DNDScheduler) FlushPending() []*model.Message {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.pending) == 0 {
		return nil
	}

	// Copy and sort by ReceivedAt
	result := make([]*model.Message, len(s.pending))
	copy(result, s.pending)
	sort.Slice(result, func(i, j int) bool {
		return result[i].ReceivedAt.Before(result[j].ReceivedAt)
	})

	// Clear the pending queue
	s.pending = make([]*model.Message, 0)
	return result
}

// PendingCount returns the current number of pending messages.
func (s *DNDScheduler) PendingCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.pending)
}

// isUrgentLevel returns true if the channel level should bypass DND.
func isUrgentLevel(level string) bool {
	return level == "critical" || level == "urgent"
}

// timeOfDayOffset returns the duration from midnight for the given time.
func timeOfDayOffset(t time.Time) time.Duration {
	hour, min, sec := t.Clock()
	return time.Duration(hour)*time.Hour + time.Duration(min)*time.Minute + time.Duration(sec)*time.Second
}

// previousWeekday returns the weekday before the given one.
func previousWeekday(wd time.Weekday) time.Weekday {
	if wd == time.Sunday {
		return time.Saturday
	}
	return wd - 1
}

// compilePeriods parses and compiles DND period configurations.
// It also merges overlapping periods with the same day mask.
func compilePeriods(periods []config.DNDPeriod) ([]CompiledDNDPeriod, error) {
	compiled := make([]CompiledDNDPeriod, 0, len(periods))

	for i, p := range periods {
		start, err := parseHHMM(p.Start)
		if err != nil {
			return nil, fmt.Errorf("dnd period %d: invalid start time %q: %w", i, p.Start, err)
		}
		end, err := parseHHMM(p.End)
		if err != nil {
			return nil, fmt.Errorf("dnd period %d: invalid end time %q: %w", i, p.End, err)
		}

		days, err := parseDays(p.Days)
		if err != nil {
			return nil, fmt.Errorf("dnd period %d: invalid days %q: %w", i, p.Days, err)
		}

		crossMidnight := start > end || (start == end && start != 0)

		compiled = append(compiled, CompiledDNDPeriod{
			Start:        start,
			End:          end,
			Days:         days,
			CrossMidnight: crossMidnight,
		})
	}

	// Merge overlapping periods
	compiled = mergePeriods(compiled)
	return compiled, nil
}

// parseHHMM parses a time string in "HH:MM" format and returns the offset from midnight.
func parseHHMM(s string) (time.Duration, error) {
	if len(s) != 5 || s[2] != ':' {
		return 0, fmt.Errorf("expected HH:MM format, got %q", s)
	}

	hour, err := parseTwoDigit(s[0:2])
	if err != nil || hour < 0 || hour > 23 {
		return 0, fmt.Errorf("invalid hour in %q", s)
	}

	min, err := parseTwoDigit(s[3:5])
	if err != nil || min < 0 || min > 59 {
		return 0, fmt.Errorf("invalid minute in %q", s)
	}

	return time.Duration(hour)*time.Hour + time.Duration(min)*time.Minute, nil
}

// parseTwoDigit parses a two-character decimal string.
func parseTwoDigit(s string) (int, error) {
	if len(s) != 2 {
		return 0, fmt.Errorf("expected 2 digits, got %q", s)
	}
	d1 := int(s[0] - '0')
	d2 := int(s[1] - '0')
	if d1 < 0 || d1 > 9 || d2 < 0 || d2 > 9 {
		return 0, fmt.Errorf("non-digit character in %q", s)
	}
	return d1*10 + d2, nil
}

// parseDays converts a day string to a DayMask.
func parseDays(s string) (DayMask, error) {
	switch s {
	case "everyday":
		return DaysEveryday, nil
	case "weekday":
		return DaysWeekday, nil
	case "weekend":
		return DaysWeekend, nil
	default:
		return 0, fmt.Errorf("unknown days value: %q (expected everyday, weekday, or weekend)", s)
	}
}

// mergePeriods merges overlapping DND periods that share the same day mask.
// For cross-midnight periods, they are normalized to a 0-24h+ range for comparison.
func mergePeriods(periods []CompiledDNDPeriod) []CompiledDNDPeriod {
	if len(periods) <= 1 {
		return periods
	}

	// Group by day mask
	groups := make(map[DayMask][]CompiledDNDPeriod)
	for _, p := range periods {
		groups[p.Days] = append(groups[p.Days], p)
	}

	var result []CompiledDNDPeriod
	for days, group := range groups {
		merged := mergePeriodsForDays(group, days)
		result = append(result, merged...)
	}

	return result
}

// interval represents a normalized time interval for merging.
type interval struct {
	start time.Duration
	end   time.Duration
}

// mergePeriodsForDays merges overlapping periods that share the same day mask.
func mergePeriodsForDays(periods []CompiledDNDPeriod, days DayMask) []CompiledDNDPeriod {
	if len(periods) <= 1 {
		return periods
	}

	// Normalize all periods to intervals where end > start
	// Cross-midnight periods: end += 24h
	intervals := make([]interval, 0, len(periods))
	for _, p := range periods {
		if p.CrossMidnight {
			intervals = append(intervals, interval{start: p.Start, end: p.End + 24*time.Hour})
		} else {
			intervals = append(intervals, interval{start: p.Start, end: p.End})
		}
	}

	// Sort by start time
	sort.Slice(intervals, func(i, j int) bool {
		return intervals[i].start < intervals[j].start
	})

	// Merge overlapping intervals
	merged := []interval{intervals[0]}
	for i := 1; i < len(intervals); i++ {
		last := &merged[len(merged)-1]
		if intervals[i].start <= last.end {
			// Overlapping or adjacent - extend
			if intervals[i].end > last.end {
				last.end = intervals[i].end
			}
		} else {
			merged = append(merged, intervals[i])
		}
	}

	// Convert back to CompiledDNDPeriod
	result := make([]CompiledDNDPeriod, 0, len(merged))
	for _, iv := range merged {
		end := iv.end
		crossMidnight := false
		if end > 24*time.Hour {
			end -= 24 * time.Hour
			crossMidnight = true
		} else if iv.start > end {
			crossMidnight = true
		}
		result = append(result, CompiledDNDPeriod{
			Start:        iv.start,
			End:          end,
			Days:         days,
			CrossMidnight: crossMidnight,
		})
	}

	return result
}
