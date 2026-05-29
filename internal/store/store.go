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
	GetDeviceByKey(deviceKey string, deviceType string) (*model.Device, error)
	CountDevicesByType(deviceType string) (int, error)

	// Channel operations
	CreateChannel(ch *model.Channel) error
	UpdateChannel(ch *model.Channel) error
	DeleteChannel(id string) error
	ListChannels() ([]model.Channel, error)
	GetChannelByName(name string) (*model.Channel, error)

	// Settings operations
	GetSetting(key string) (string, error)
	SetSetting(key string, value string) error
	GetSettings(prefix string) (map[string]string, error)

	// Auth operations
	CreateUser(u *model.User) error
	GetUserByUsername(username string) (*model.User, error)
	GetUserCount() (int, error)
	UpdateUserPassword(id string, passwordHash string) error

	// API Key operations
	CreateAPIKey(key *model.APIKey) error
	ListAPIKeys(userID string) ([]model.APIKey, error)
	DeleteAPIKey(id string) error
	GetAPIKeyByHash(keyHash string) (*model.APIKey, error)
	UpdateAPIKeyLastUsed(id string) error
	ValidateAPIKey(rawKey string) (*model.APIKey, error)

	// Notification lifecycle operations
	AckMessage(id string, ackAt time.Time) error
	SnoozeMessage(id string, snoozeUntil time.Time) error
	GetMessage(id string) (*model.Message, error)
	GetUnackedMessages(channelNames []string) ([]model.Message, error)
	GetCatchUpMessages(lastSeen time.Time, maxAge time.Duration) ([]model.Message, error)
	UpdateRepeatCount(id string, count int) error
	ExpireMessage(id string) error
	UpdateDeviceLastSeen(deviceKey string, lastSeen time.Time) error
	GetDeviceLastSeen(deviceKey string) (time.Time, error)

	// Lifecycle
	Close() error
	Migrate() error
}
