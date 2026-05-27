package model

// ScheduleConfig is the unified scheduling configuration.
// Ported from taskdog's schedule_config.py.
type ScheduleConfig struct {
	Type      string         `json:"type"`                 // once/interval/crontab/daily/weekly/monthly/yearly/workday/weekend
	Config    map[string]any `json:"config"`               // type-specific config
	Timezone  string         `json:"timezone,omitempty"`   // timezone name, default "Asia/Shanghai"
	StartTime *string        `json:"start_time,omitempty"` // effective start time (ISO)
	EndTime   *string        `json:"end_time,omitempty"`   // effective end time (ISO)
	RawInput  string         `json:"raw_input,omitempty"`  // original natural language input
}

// Valid schedule types
var ValidScheduleTypes = map[string]bool{
	"once":     true,
	"interval": true,
	"crontab":  true,
	"daily":    true,
	"weekly":   true,
	"monthly":  true,
	"yearly":   true,
	"workday":  true,
	"weekend":  true,
}

// OnceConfig: {"datetime": "2024-01-15T09:00:00+08:00"}
// IntervalConfig: {"period": 3600} (seconds)
// CrontabConfig: {"expression": "0 9 * * *"}
// DailyConfig: {"time": "09:00:00"}
// WeeklyConfig: {"days": [1,3,5], "time": "09:00:00"} (1=Monday, 7=Sunday)
// MonthlyConfig: {"days": [1,15], "time": "09:00:00"}
// YearlyConfig: {"month": 1, "day": 15, "time": "09:00:00"}
// WorkdayConfig: {"time": "09:00:00"}
// WeekendConfig: {"time": "09:00:00"}
