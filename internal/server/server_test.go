package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/overseer/overseer/internal/config"
	"github.com/overseer/overseer/internal/model"
	"github.com/overseer/overseer/internal/store"
	"github.com/stretchr/testify/assert"
)

// testStore is a minimal mock store for server tests.
type testStore struct{}

func (s *testStore) SaveMessage(_ *model.Message) error                              { return nil }
func (s *testStore) UpdateMessageStatus(_ string, _ model.PushStatus, _ string) error { return nil }
func (s *testStore) QueryMessages(_ store.MessageFilter) (*model.PagedResult[model.Message], error) {
	return nil, nil
}
func (s *testStore) GetChannelStats(_, _ time.Time) ([]model.ChannelStats, error) { return nil, nil }
func (s *testStore) CreateReminder(_ *model.Reminder) error                        { return nil }
func (s *testStore) UpdateReminder(_ *model.Reminder) error                        { return nil }
func (s *testStore) CancelReminder(_ string) error                                 { return nil }
func (s *testStore) ListReminders(_ store.ReminderFilter) ([]model.Reminder, error) {
	return nil, nil
}
func (s *testStore) GetActiveReminders() ([]model.Reminder, error)  { return nil, nil }
func (s *testStore) UpdateNextTrigger(_ string, _ time.Time) error  { return nil }
func (s *testStore) CreateDevice(_ *model.Device) error             { return nil }
func (s *testStore) UpdateDevice(_ *model.Device) error             { return nil }
func (s *testStore) DeleteDevice(_ string) error                    { return nil }
func (s *testStore) ListDevices() ([]model.Device, error)           { return nil, nil }
func (s *testStore) GetDefaultDevice() (*model.Device, error)       { return nil, nil }
func (s *testStore) SetDefaultDevice(_ string) error                { return nil }
func (s *testStore) GetDeviceByKey(_ string, _ string) (*model.Device, error) { return nil, nil }
func (s *testStore) CountDevicesByType(_ string) (int, error)       { return 0, nil }
func (s *testStore) CreateChannel(_ *model.Channel) error           { return nil }
func (s *testStore) UpdateChannel(_ *model.Channel) error           { return nil }
func (s *testStore) DeleteChannel(_ string) error                   { return nil }
func (s *testStore) ListChannels() ([]model.Channel, error)         { return nil, nil }
func (s *testStore) GetChannelByName(_ string) (*model.Channel, error) { return nil, nil }
func (s *testStore) GetSetting(_ string) (string, error)               { return "", nil }
func (s *testStore) SetSetting(_ string, _ string) error               { return nil }
func (s *testStore) GetSettings(_ string) (map[string]string, error)   { return nil, nil }
func (s *testStore) CreateUser(_ *model.User) error                    { return nil }
func (s *testStore) GetUserByUsername(_ string) (*model.User, error)   { return nil, nil }
func (s *testStore) GetUserCount() (int, error)                        { return 0, nil }
func (s *testStore) UpdateUserPassword(_ string, _ string) error       { return nil }
func (s *testStore) CreateAPIKey(_ *model.APIKey) error                { return nil }
func (s *testStore) ListAPIKeys(_ string) ([]model.APIKey, error)      { return nil, nil }
func (s *testStore) DeleteAPIKey(_ string) error                       { return nil }
func (s *testStore) GetAPIKeyByHash(_ string) (*model.APIKey, error)   { return nil, nil }
func (s *testStore) UpdateAPIKeyLastUsed(_ string) error               { return nil }
func (s *testStore) ValidateAPIKey(_ string) (*model.APIKey, error)    { return nil, nil }
func (s *testStore) Close() error                                      { return nil }
func (s *testStore) Migrate() error                                    { return nil }

func testConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Port:      8080,
			JWTSecret: "test-jwt-secret",
		},
	}
}

func TestNewServer(t *testing.T) {
	cfg := testConfig()
	s := NewServer(cfg, &testStore{})

	assert.NotNil(t, s)
	assert.NotNil(t, s.engine)
	assert.Equal(t, cfg, s.cfg)
}

func TestServer_HealthEndpoint(t *testing.T) {
	cfg := testConfig()
	s := NewServer(cfg, &testStore{})
	s.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	s.engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, float64(200), resp["code"])
	assert.Equal(t, "ok", resp["message"])
}

func TestServer_HealthEndpointNoAuthRequired(t *testing.T) {
	cfg := testConfig()
	s := NewServer(cfg, &testStore{})
	s.SetupRoutes()

	// No auth - should still work for /health
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	s.engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestServer_Shutdown(t *testing.T) {
	cfg := testConfig()
	s := NewServer(cfg, &testStore{})
	s.SetupRoutes()

	// Shutdown without Start should not error
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := s.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestServer_Engine(t *testing.T) {
	cfg := testConfig()
	s := NewServer(cfg, &testStore{})

	assert.NotNil(t, s.Engine())
	assert.Equal(t, s.engine, s.Engine())
}
