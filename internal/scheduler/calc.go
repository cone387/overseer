package scheduler

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/overseer/overseer/internal/model"
	"github.com/robfig/cron/v3"
)

// CalcNextTime calculates the next execution time based on schedule config.
// Returns nil if no more executions (e.g., once type after first trigger, or past end_time).
func CalcNextTime(sc *model.ScheduleConfig, fromTime time.Time) *time.Time {
	if sc == nil {
		return nil
	}

	tz := loadTimezone(sc.Timezone)
	fromLocal := fromTime.In(tz)

	var nextTime *time.Time

	switch sc.Type {
	case "once":
		// One-time execution: return the configured datetime if it's in the future
		dtStr, _ := sc.Config["datetime"].(string)
		if dtStr == "" {
			return nil
		}
		dt, err := time.Parse(time.RFC3339, dtStr)
		if err != nil {
			return nil
		}
		if dt.After(fromTime) {
			nextTime = &dt
		}
		return nextTime

	case "interval":
		period := getIntConfig(sc.Config, "period")
		if period <= 0 {
			return nil
		}
		next := fromTime.Add(time.Duration(period) * time.Second)
		nextTime = &next

	case "crontab":
		expr, _ := sc.Config["expression"].(string)
		if expr == "" {
			return nil
		}
		// Use robfig/cron parser with seconds support
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		schedule, err := parser.Parse(expr)
		if err != nil {
			// Try with seconds
			parser2 := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
			schedule, err = parser2.Parse(expr)
			if err != nil {
				return nil
			}
		}
		next := schedule.Next(fromLocal)
		nextUTC := next.UTC()
		nextTime = &nextUTC

	case "daily":
		targetTime := parseTimeStr(getStrConfig(sc.Config, "time"))
		if targetTime == nil {
			return nil
		}
		next := fromLocal
		next = time.Date(next.Year(), next.Month(), next.Day(), targetTime.Hour(), targetTime.Minute(), targetTime.Second(), 0, tz)
		if !next.After(fromLocal) {
			next = next.AddDate(0, 0, 1)
		}
		nextUTC := next.UTC()
		nextTime = &nextUTC

	case "weekly":
		days := getIntSliceConfig(sc.Config, "days")
		targetTime := parseTimeStr(getStrConfig(sc.Config, "time"))
		if targetTime == nil || len(days) == 0 {
			return nil
		}
		// Check today first, then next 7 days
		for i := 0; i < 8; i++ {
			candidate := fromLocal.AddDate(0, 0, i)
			candidate = time.Date(candidate.Year(), candidate.Month(), candidate.Day(), targetTime.Hour(), targetTime.Minute(), targetTime.Second(), 0, tz)
			isoWeekday := int(candidate.Weekday())
			if isoWeekday == 0 {
				isoWeekday = 7 // Sunday = 7
			}
			if contains(days, isoWeekday) && candidate.After(fromLocal) {
				nextUTC := candidate.UTC()
				nextTime = &nextUTC
				break
			}
		}

	case "monthly":
		days := getIntSliceConfig(sc.Config, "days")
		targetTime := parseTimeStr(getStrConfig(sc.Config, "time"))
		if targetTime == nil || len(days) == 0 {
			return nil
		}
		for i := 0; i < 62; i++ {
			candidate := fromLocal.AddDate(0, 0, i)
			candidate = time.Date(candidate.Year(), candidate.Month(), candidate.Day(), targetTime.Hour(), targetTime.Minute(), targetTime.Second(), 0, tz)
			if contains(days, candidate.Day()) && candidate.After(fromLocal) {
				nextUTC := candidate.UTC()
				nextTime = &nextUTC
				break
			}
		}

	case "yearly":
		month := getIntConfig(sc.Config, "month")
		day := getIntConfig(sc.Config, "day")
		targetTime := parseTimeStr(getStrConfig(sc.Config, "time"))
		if targetTime == nil || month == 0 || day == 0 {
			return nil
		}
		// Try this year
		candidate := time.Date(fromLocal.Year(), time.Month(month), day, targetTime.Hour(), targetTime.Minute(), targetTime.Second(), 0, tz)
		if !candidate.After(fromLocal) {
			candidate = time.Date(fromLocal.Year()+1, time.Month(month), day, targetTime.Hour(), targetTime.Minute(), targetTime.Second(), 0, tz)
		}
		nextUTC := candidate.UTC()
		nextTime = &nextUTC

	case "workday":
		targetTime := parseTimeStr(getStrConfig(sc.Config, "time"))
		if targetTime == nil {
			return nil
		}
		for i := 0; i < 8; i++ {
			candidate := fromLocal.AddDate(0, 0, i)
			candidate = time.Date(candidate.Year(), candidate.Month(), candidate.Day(), targetTime.Hour(), targetTime.Minute(), targetTime.Second(), 0, tz)
			wd := candidate.Weekday()
			if wd >= time.Monday && wd <= time.Friday && candidate.After(fromLocal) {
				nextUTC := candidate.UTC()
				nextTime = &nextUTC
				break
			}
		}

	case "weekend":
		targetTime := parseTimeStr(getStrConfig(sc.Config, "time"))
		if targetTime == nil {
			return nil
		}
		for i := 0; i < 8; i++ {
			candidate := fromLocal.AddDate(0, 0, i)
			candidate = time.Date(candidate.Year(), candidate.Month(), candidate.Day(), targetTime.Hour(), targetTime.Minute(), targetTime.Second(), 0, tz)
			wd := candidate.Weekday()
			if (wd == time.Saturday || wd == time.Sunday) && candidate.After(fromLocal) {
				nextUTC := candidate.UTC()
				nextTime = &nextUTC
				break
			}
		}
	}

	// Check end_time boundary
	if nextTime != nil && sc.EndTime != nil {
		endDt, err := time.Parse(time.RFC3339, *sc.EndTime)
		if err == nil && nextTime.After(endDt) {
			return nil
		}
	}

	return nextTime
}

// --- Helpers ---

func loadTimezone(name string) *time.Location {
	if name == "" {
		name = "Asia/Shanghai"
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return loc
}

type parsedTime struct {
	hour, minute, second int
}

func (p *parsedTime) Hour() int   { return p.hour }
func (p *parsedTime) Minute() int { return p.minute }
func (p *parsedTime) Second() int { return p.second }

func parseTimeStr(s string) *parsedTime {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ":")
	if len(parts) < 2 {
		return nil
	}
	h, _ := strconv.Atoi(parts[0])
	m, _ := strconv.Atoi(parts[1])
	sec := 0
	if len(parts) > 2 {
		sec, _ = strconv.Atoi(parts[2])
	}
	return &parsedTime{hour: h, minute: m, second: sec}
}

func getStrConfig(config map[string]any, key string) string {
	v, ok := config[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func getIntConfig(config map[string]any, key string) int {
	v, ok := config[key]
	if !ok {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return int(val)
	case int:
		return val
	case string:
		n, _ := strconv.Atoi(val)
		return n
	}
	return 0
}

func getIntSliceConfig(config map[string]any, key string) []int {
	v, ok := config[key]
	if !ok {
		return nil
	}
	switch val := v.(type) {
	case []any:
		result := make([]int, 0, len(val))
		for _, item := range val {
			switch n := item.(type) {
			case float64:
				result = append(result, int(n))
			case int:
				result = append(result, n)
			}
		}
		return result
	case []int:
		return val
	}
	return nil
}

func contains(slice []int, val int) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}
