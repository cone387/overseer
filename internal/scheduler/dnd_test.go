package scheduler

import (
	"fmt"
	"testing"
	"time"

	"github.com/overseer/overseer/internal/config"
	"github.com/overseer/overseer/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper to create a time on a specific weekday at HH:MM
func makeTime(weekday time.Weekday, hour, min int) time.Time {
	// Use a known date: 2024-01-01 is Monday
	// Monday=1, so offset from Monday
	baseMonday := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	daysFromMonday := int(weekday) - int(time.Monday)
	if daysFromMonday < 0 {
		daysFromMonday += 7
	}
	return baseMonday.AddDate(0, 0, daysFromMonday).Add(
		time.Duration(hour)*time.Hour + time.Duration(min)*time.Minute,
	)
}

func TestParseHHMM(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{"00:00", 0, false},
		{"23:59", 23*time.Hour + 59*time.Minute, false},
		{"07:30", 7*time.Hour + 30*time.Minute, false},
		{"12:00", 12 * time.Hour, false},
		{"", 0, true},
		{"7:30", 0, true},
		{"25:00", 0, true},
		{"12:60", 0, true},
		{"ab:cd", 0, true},
		{"12-00", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseHHMM(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, got)
			}
		})
	}
}

func TestParseDays(t *testing.T) {
	tests := []struct {
		input    string
		expected DayMask
		wantErr  bool
	}{
		{"everyday", DaysEveryday, false},
		{"weekday", DaysWeekday, false},
		{"weekend", DaysWeekend, false},
		{"invalid", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseDays(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, got)
			}
		})
	}
}

func TestNewDNDScheduler_InvalidConfig(t *testing.T) {
	tests := []struct {
		name    string
		periods []config.DNDPeriod
	}{
		{
			name:    "invalid start time",
			periods: []config.DNDPeriod{{Start: "25:00", End: "07:00", Days: "everyday"}},
		},
		{
			name:    "invalid end time",
			periods: []config.DNDPeriod{{Start: "23:00", End: "99:99", Days: "everyday"}},
		},
		{
			name:    "invalid days",
			periods: []config.DNDPeriod{{Start: "23:00", End: "07:00", Days: "never"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewDNDScheduler(tt.periods, 200)
			assert.Error(t, err)
		})
	}
}

func TestNewDNDScheduler_DefaultMaxPending(t *testing.T) {
	s, err := NewDNDScheduler(nil, 0)
	require.NoError(t, err)
	assert.Equal(t, DefaultMaxPending, s.maxPending)
}

func TestShouldDefer_SameDayPeriod(t *testing.T) {
	// DND from 09:00 to 12:00 on weekends
	periods := []config.DNDPeriod{
		{Start: "09:00", End: "12:00", Days: "weekend"},
	}
	s, err := NewDNDScheduler(periods, 200)
	require.NoError(t, err)

	// Saturday 10:00 - should defer non-urgent
	sat10 := makeTime(time.Saturday, 10, 0)
	assert.True(t, s.ShouldDefer("active", sat10))
	assert.True(t, s.ShouldDefer("timeSensitive", sat10))
	assert.True(t, s.ShouldDefer("passive", sat10))

	// Saturday 10:00 - urgent should NOT defer
	assert.False(t, s.ShouldDefer("urgent", sat10))
	assert.False(t, s.ShouldDefer("critical", sat10))

	// Saturday 08:00 - before DND, should not defer
	sat08 := makeTime(time.Saturday, 8, 0)
	assert.False(t, s.ShouldDefer("active", sat08))

	// Saturday 12:00 - at end boundary (exclusive), should not defer
	sat12 := makeTime(time.Saturday, 12, 0)
	assert.False(t, s.ShouldDefer("active", sat12))

	// Monday 10:00 - weekday, not in weekend DND
	mon10 := makeTime(time.Monday, 10, 0)
	assert.False(t, s.ShouldDefer("active", mon10))
}

func TestShouldDefer_CrossMidnightPeriod(t *testing.T) {
	// DND from 23:00 to 07:30 everyday
	periods := []config.DNDPeriod{
		{Start: "23:00", End: "07:30", Days: "everyday"},
	}
	s, err := NewDNDScheduler(periods, 200)
	require.NoError(t, err)

	// Monday 23:30 - in evening portion
	mon2330 := makeTime(time.Monday, 23, 30)
	assert.True(t, s.ShouldDefer("active", mon2330))

	// Tuesday 06:00 - in morning portion (started Monday night)
	tue06 := makeTime(time.Tuesday, 6, 0)
	assert.True(t, s.ShouldDefer("active", tue06))

	// Tuesday 08:00 - after DND ends
	tue08 := makeTime(time.Tuesday, 8, 0)
	assert.False(t, s.ShouldDefer("active", tue08))

	// Monday 22:00 - before DND starts
	mon22 := makeTime(time.Monday, 22, 0)
	assert.False(t, s.ShouldDefer("active", mon22))
}

func TestShouldDefer_CrossMidnight_WeekdayOnly(t *testing.T) {
	// DND from 22:00 to 06:00 on weekdays only
	periods := []config.DNDPeriod{
		{Start: "22:00", End: "06:00", Days: "weekday"},
	}
	s, err := NewDNDScheduler(periods, 200)
	require.NoError(t, err)

	// Friday 23:00 - Friday is weekday, should defer
	fri23 := makeTime(time.Friday, 23, 0)
	assert.True(t, s.ShouldDefer("active", fri23))

	// Saturday 03:00 - morning portion, started Friday (weekday), should defer
	sat03 := makeTime(time.Saturday, 3, 0)
	assert.True(t, s.ShouldDefer("active", sat03))

	// Saturday 23:00 - Saturday is not a weekday, should NOT defer
	sat23 := makeTime(time.Saturday, 23, 0)
	assert.False(t, s.ShouldDefer("active", sat23))

	// Sunday 03:00 - morning portion, started Saturday (not weekday), should NOT defer
	sun03 := makeTime(time.Sunday, 3, 0)
	assert.False(t, s.ShouldDefer("active", sun03))
}

func TestShouldDefer_EverydayPeriod(t *testing.T) {
	periods := []config.DNDPeriod{
		{Start: "01:00", End: "05:00", Days: "everyday"},
	}
	s, err := NewDNDScheduler(periods, 200)
	require.NoError(t, err)

	// Any day at 03:00 should defer
	for wd := time.Sunday; wd <= time.Saturday; wd++ {
		t03 := makeTime(wd, 3, 0)
		assert.True(t, s.ShouldDefer("active", t03), "expected defer on %s at 03:00", wd)
	}
}

func TestShouldDefer_UrgentAlwaysPasses(t *testing.T) {
	// DND all day everyday
	periods := []config.DNDPeriod{
		{Start: "00:00", End: "23:59", Days: "everyday"},
	}
	s, err := NewDNDScheduler(periods, 200)
	require.NoError(t, err)

	now := makeTime(time.Wednesday, 12, 0)
	assert.False(t, s.ShouldDefer("urgent", now))
	assert.False(t, s.ShouldDefer("critical", now))
	assert.True(t, s.ShouldDefer("active", now))
}

func TestEnqueue_Basic(t *testing.T) {
	s, err := NewDNDScheduler(nil, 200)
	require.NoError(t, err)

	msg := &model.Message{ID: "1", ReceivedAt: time.Now()}
	s.Enqueue(msg)
	assert.Equal(t, 1, s.PendingCount())
}

func TestEnqueue_MaxPendingEviction(t *testing.T) {
	s, err := NewDNDScheduler(nil, 3)
	require.NoError(t, err)

	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		msg := &model.Message{
			ID:         fmt.Sprintf("msg-%d", i),
			ReceivedAt: base.Add(time.Duration(i) * time.Minute),
		}
		s.Enqueue(msg)
	}

	// Should only have 3 messages (the last 3)
	assert.Equal(t, 3, s.PendingCount())

	flushed := s.FlushPending()
	require.Len(t, flushed, 3)
	// Should be msg-2, msg-3, msg-4 (oldest two were evicted)
	assert.Equal(t, "msg-2", flushed[0].ID)
	assert.Equal(t, "msg-3", flushed[1].ID)
	assert.Equal(t, "msg-4", flushed[2].ID)
}

func TestFlushPending_SortsByReceivedAt(t *testing.T) {
	s, err := NewDNDScheduler(nil, 200)
	require.NoError(t, err)

	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	// Enqueue in reverse order
	for i := 4; i >= 0; i-- {
		msg := &model.Message{
			ID:         fmt.Sprintf("msg-%d", i),
			ReceivedAt: base.Add(time.Duration(i) * time.Minute),
		}
		s.Enqueue(msg)
	}

	flushed := s.FlushPending()
	require.Len(t, flushed, 5)

	// Should be sorted by ReceivedAt ascending
	for i := 0; i < 5; i++ {
		assert.Equal(t, fmt.Sprintf("msg-%d", i), flushed[i].ID)
	}
}

func TestFlushPending_ClearsQueue(t *testing.T) {
	s, err := NewDNDScheduler(nil, 200)
	require.NoError(t, err)

	msg := &model.Message{ID: "1", ReceivedAt: time.Now()}
	s.Enqueue(msg)

	flushed := s.FlushPending()
	assert.Len(t, flushed, 1)
	assert.Equal(t, 0, s.PendingCount())

	// Second flush should return nil
	flushed2 := s.FlushPending()
	assert.Nil(t, flushed2)
}

func TestFlushPending_Empty(t *testing.T) {
	s, err := NewDNDScheduler(nil, 200)
	require.NoError(t, err)

	flushed := s.FlushPending()
	assert.Nil(t, flushed)
}

func TestOverlappingPeriods_Merged(t *testing.T) {
	// Two overlapping periods on everyday: 22:00-02:00 and 01:00-07:00
	// Should merge into 22:00-07:00
	periods := []config.DNDPeriod{
		{Start: "22:00", End: "02:00", Days: "everyday"},
		{Start: "01:00", End: "07:00", Days: "everyday"},
	}
	s, err := NewDNDScheduler(periods, 200)
	require.NoError(t, err)

	// 23:00 - in the merged period
	t2300 := makeTime(time.Monday, 23, 0)
	assert.True(t, s.ShouldDefer("active", t2300))

	// 03:00 - in the merged period (morning portion)
	t0300 := makeTime(time.Tuesday, 3, 0)
	assert.True(t, s.ShouldDefer("active", t0300))

	// 05:00 - still in merged period
	t0500 := makeTime(time.Tuesday, 5, 0)
	assert.True(t, s.ShouldDefer("active", t0500))

	// 08:00 - after merged period ends
	t0800 := makeTime(time.Tuesday, 8, 0)
	assert.False(t, s.ShouldDefer("active", t0800))
}

func TestOverlappingPeriods_DifferentDays_NotMerged(t *testing.T) {
	// Weekday 22:00-06:00 and weekend 23:00-08:00 should NOT merge
	periods := []config.DNDPeriod{
		{Start: "22:00", End: "06:00", Days: "weekday"},
		{Start: "23:00", End: "08:00", Days: "weekend"},
	}
	s, err := NewDNDScheduler(periods, 200)
	require.NoError(t, err)

	// Monday 23:00 - weekday DND active
	mon23 := makeTime(time.Monday, 23, 0)
	assert.True(t, s.ShouldDefer("active", mon23))

	// Saturday 23:30 - weekend DND active
	sat2330 := makeTime(time.Saturday, 23, 30)
	assert.True(t, s.ShouldDefer("active", sat2330))

	// Saturday 07:00 - weekend morning portion (started Friday which is weekday)
	// Friday is a weekday, so weekday DND 22:00-06:00 applies
	// Saturday 07:00 > 06:00, so weekday DND doesn't apply
	// But weekend DND 23:00-08:00: yesterday is Friday (not weekend), so morning portion doesn't apply
	sat07 := makeTime(time.Saturday, 7, 0)
	assert.False(t, s.ShouldDefer("active", sat07))
}

func TestNoPeriods(t *testing.T) {
	s, err := NewDNDScheduler(nil, 200)
	require.NoError(t, err)

	// No DND periods means nothing is deferred
	now := makeTime(time.Monday, 12, 0)
	assert.False(t, s.ShouldDefer("active", now))
}

func TestAdjacentPeriods_Merged(t *testing.T) {
	// Adjacent periods: 09:00-12:00 and 12:00-15:00 on everyday
	periods := []config.DNDPeriod{
		{Start: "09:00", End: "12:00", Days: "everyday"},
		{Start: "12:00", End: "15:00", Days: "everyday"},
	}
	s, err := NewDNDScheduler(periods, 200)
	require.NoError(t, err)

	// 11:00 - in first period
	t1100 := makeTime(time.Monday, 11, 0)
	assert.True(t, s.ShouldDefer("active", t1100))

	// 13:00 - in second period (merged)
	t1300 := makeTime(time.Monday, 13, 0)
	assert.True(t, s.ShouldDefer("active", t1300))

	// 15:00 - at end boundary (exclusive)
	t1500 := makeTime(time.Monday, 15, 0)
	assert.False(t, s.ShouldDefer("active", t1500))
}


