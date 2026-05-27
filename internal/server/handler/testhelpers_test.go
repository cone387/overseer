package handler

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/overseer/overseer/internal/model"
	"github.com/overseer/overseer/internal/store"
	"github.com/stretchr/testify/mock"
)

// mockStore implements store.Store for testing using testify/mock.
type mockStore struct {
	mock.Mock
}

func (m *mockStore) SaveMessage(msg *model.Message) error {
	args := m.Called(msg)
	return args.Error(0)
}

func (m *mockStore) UpdateMessageStatus(id string, status model.PushStatus, failReason string) error {
	args := m.Called(id, status, failReason)
	return args.Error(0)
}

func (m *mockStore) QueryMessages(filter store.MessageFilter) (*model.PagedResult[model.Message], error) {
	args := m.Called(filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.PagedResult[model.Message]), args.Error(1)
}

func (m *mockStore) GetChannelStats(from, to time.Time) ([]model.ChannelStats, error) {
	args := m.Called(from, to)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]model.ChannelStats), args.Error(1)
}

func (m *mockStore) CreateReminder(r *model.Reminder) error {
	args := m.Called(r)
	return args.Error(0)
}

func (m *mockStore) UpdateReminder(r *model.Reminder) error {
	args := m.Called(r)
	return args.Error(0)
}

func (m *mockStore) CancelReminder(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *mockStore) ListReminders(filter store.ReminderFilter) ([]model.Reminder, error) {
	args := m.Called(filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]model.Reminder), args.Error(1)
}

func (m *mockStore) GetActiveReminders() ([]model.Reminder, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]model.Reminder), args.Error(1)
}

func (m *mockStore) UpdateNextTrigger(id string, next time.Time) error {
	args := m.Called(id, next)
	return args.Error(0)
}

func (m *mockStore) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockStore) Migrate() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockStore) CreateDevice(d *model.Device) error {
	args := m.Called(d)
	return args.Error(0)
}

func (m *mockStore) UpdateDevice(d *model.Device) error {
	args := m.Called(d)
	return args.Error(0)
}

func (m *mockStore) DeleteDevice(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *mockStore) ListDevices() ([]model.Device, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]model.Device), args.Error(1)
}

func (m *mockStore) GetDefaultDevice() (*model.Device, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Device), args.Error(1)
}

func (m *mockStore) SetDefaultDevice(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *mockStore) CreateChannel(ch *model.Channel) error {
	args := m.Called(ch)
	return args.Error(0)
}

func (m *mockStore) UpdateChannel(ch *model.Channel) error {
	args := m.Called(ch)
	return args.Error(0)
}

func (m *mockStore) DeleteChannel(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *mockStore) ListChannels() ([]model.Channel, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]model.Channel), args.Error(1)
}

func (m *mockStore) GetChannelByName(name string) (*model.Channel, error) {
	args := m.Called(name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Channel), args.Error(1)
}

func (m *mockStore) GetSetting(key string) (string, error) {
	args := m.Called(key)
	return args.String(0), args.Error(1)
}

func (m *mockStore) SetSetting(key string, value string) error {
	args := m.Called(key, value)
	return args.Error(0)
}

func (m *mockStore) GetSettings(prefix string) (map[string]string, error) {
	args := m.Called(prefix)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]string), args.Error(1)
}

// --- Auth mock methods ---

func (m *mockStore) CreateUser(u *model.User) error {
	args := m.Called(u)
	return args.Error(0)
}

func (m *mockStore) GetUserByUsername(username string) (*model.User, error) {
	args := m.Called(username)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.User), args.Error(1)
}

func (m *mockStore) GetUserCount() (int, error) {
	args := m.Called()
	return args.Int(0), args.Error(1)
}

func (m *mockStore) UpdateUserPassword(id string, passwordHash string) error {
	args := m.Called(id, passwordHash)
	return args.Error(0)
}

func (m *mockStore) CreateAPIKey(key *model.APIKey) error {
	args := m.Called(key)
	return args.Error(0)
}

func (m *mockStore) ListAPIKeys(userID string) ([]model.APIKey, error) {
	args := m.Called(userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]model.APIKey), args.Error(1)
}

func (m *mockStore) DeleteAPIKey(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *mockStore) GetAPIKeyByHash(keyHash string) (*model.APIKey, error) {
	args := m.Called(keyHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.APIKey), args.Error(1)
}

func (m *mockStore) UpdateAPIKeyLastUsed(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *mockStore) ValidateAPIKey(rawKey string) (*model.APIKey, error) {
	args := m.Called(rawKey)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.APIKey), args.Error(1)
}

// setupRouter creates a Gin engine with the webhook handler registered.
// Used by webhook_test.go.
func setupRouter(handler MessageHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	wh := NewWebhookHandler(handler)
	r.POST("/webhook/:source", wh.HandleWebhook)
	return r
}
