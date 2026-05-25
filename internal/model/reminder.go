package model

import "time"

// RepeatType defines how a reminder repeats.
type RepeatType string

const (
	RepeatOnce   RepeatType = "once"
	RepeatDaily  RepeatType = "daily"
	RepeatWeekly RepeatType = "weekly"
	RepeatCron   RepeatType = "cron"
)

// Reminder represents a scheduled reminder that triggers push notifications.
type Reminder struct {
	ID            string     `json:"id"`
	Title         string     `json:"title"`
	Body          string     `json:"body,omitempty"`
	Channel       string     `json:"channel"`
	TriggerAt     time.Time  `json:"trigger_at"`
	RepeatType    RepeatType `json:"repeat_type"`
	RepeatRule    string     `json:"repeat_rule,omitempty"`
	Status        string     `json:"status"`
	NextTrigger   *time.Time `json:"next_trigger,omitempty"`
	LastTriggered *time.Time `json:"last_triggered,omitempty"`
	FailReason    string     `json:"fail_reason,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
