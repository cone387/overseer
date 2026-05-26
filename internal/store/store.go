package store

import (
	"time"

	"github.com/overseer/overseer/internal/model"
)

// MessageFilter defines query parameters for filtering messages.
type MessageFilter struct {
	Channel  string
	Status   model.PushStatus
	From     time.Time
	To       time.Time
	Page     int
	PageSize int
}

// ReminderFilter defines query parameters for filtering reminders.
type ReminderFilter struct {
	Status string
}

// Store is the persistence interface for messages and reminders.
type Store interface {
	// Message operations
	SaveMessage(msg *model.Message) error
	UpdateMessageStatus(id string, status model.PushStatus, failReason string) error
	QueryMessages(filter MessageFilter) (*model.PagedResult[model.Message], error)
	GetChannelStats(from, to time.Time) ([]model.ChannelStats, error)

	// Reminder operations
	CreateReminder(r *model.Reminder) error
	UpdateReminder(r *model.Reminder) error
	CancelReminder(id string) error
	ListReminders(filter ReminderFilter) ([]model.Reminder, error)
	GetActiveReminders() ([]model.Reminder, error)
	UpdateNextTrigger(id string, next time.Time) error

	// Device operations
	CreateDevice(d *model.Device) error
	UpdateDevice(d *model.Device) error
	DeleteDevice(id string) error
	ListDevices() ([]model.Device, error)
	GetDefaultDevice() (*model.Device, error)
	SetDefaultDevice(id string) error

	// Channel operations
	CreateChannel(ch *model.Channel) error
	UpdateChannel(ch *model.Channel) error
	DeleteChannel(id string) error
	ListChannels() ([]model.Channel, error)
	GetChannelByName(name string) (*model.Channel, error)

	// Lifecycle
	Close() error
	Migrate() error
}
