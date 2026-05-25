package pusher

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/overseer/overseer/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBarkPusher(t *testing.T) {
	cfg := config.BarkConfig{
		ServerURL:  "https://api.day.app",
		DeviceKey:  "test-key",
		Timeout:    10,
		MaxRetries: 3,
	}
	p := NewBarkPusher(cfg)
	assert.Equal(t, "https://api.day.app", p.serverURL)
	assert.Equal(t, 3, p.maxRetries)
	assert.Equal(t, 10*time.Second, p.httpClient.Timeout)
}

func TestNewBarkPusher_Defaults(t *testing.T) {
	cfg := config.BarkConfig{
		ServerURL: "https://api.day.app",
		DeviceKey: "test-key",
	}
	p := NewBarkPusher(cfg)
	assert.Equal(t, 10*time.Second, p.httpClient.Timeout)
	assert.Equal(t, 3, p.maxRetries)
}

func TestBarkPusher_Name(t *testing.T) {
	p := NewBarkPusher(config.BarkConfig{ServerURL: "http://localhost"})
	assert.Equal(t, "bark", p.Name())
}

func TestBarkPusher_Push_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/test-device-key", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var body barkRequestBody
		err := json.NewDecoder(r.Body).Decode(&body)
		require.NoError(t, err)
		assert.Equal(t, "Test Title", body.Title)
		assert.Equal(t, "Test Body", body.Body)
		assert.Equal(t, "alarm.caf", body.Sound)
		assert.Equal(t, "test-group", body.Group)
		assert.Equal(t, "https://example.com/icon.png", body.Icon)
		assert.Equal(t, "active", body.Level)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(barkResponse{
			Code:      200,
			Message:   "success",
			Timestamp: 1234567890,
		})
	}))
	defer server.Close()

	p := NewBarkPusher(config.BarkConfig{
		ServerURL:  server.URL,
		Timeout:    5,
		MaxRetries: 3,
	})

	resp, err := p.Push(context.Background(), PushRequest{
		DeviceKey: "test-device-key",
		Title:     "Test Title",
		Body:      "Test Body",
		Sound:     "alarm.caf",
		Group:     "test-group",
		Icon:      "https://example.com/icon.png",
		Level:     "active",
	})

	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, "1234567890", resp.MessageID)
	assert.Empty(t, resp.Error)
}

func TestBarkPusher_Push_EmptyDeviceKey(t *testing.T) {
	p := NewBarkPusher(config.BarkConfig{
		ServerURL:  "http://localhost",
		Timeout:    5,
		MaxRetries: 3,
	})

	resp, err := p.Push(context.Background(), PushRequest{
		DeviceKey: "",
		Title:     "Test",
		Body:      "Test",
	})

	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Equal(t, "device_key is empty", resp.Error)
}

func TestBarkPusher_Push_HTTPError(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	p := &BarkPusher{
		serverURL:  server.URL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
		maxRetries: 2,
	}

	resp, err := p.Push(context.Background(), PushRequest{
		DeviceKey: "test-key",
		Title:     "Test",
		Body:      "Test",
	})

	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Error, "HTTP 500")
	// 1 initial + 2 retries = 3 total attempts
	assert.Equal(t, int32(3), atomic.LoadInt32(&attempts))
}

func TestBarkPusher_Push_BarkErrorCode(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(barkResponse{
			Code:    400,
			Message: "device not found",
		})
	}))
	defer server.Close()

	p := &BarkPusher{
		serverURL:  server.URL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
		maxRetries: 2,
	}

	resp, err := p.Push(context.Background(), PushRequest{
		DeviceKey: "invalid-key",
		Title:     "Test",
		Body:      "Test",
	})

	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Error, "bark error code 400")
	assert.Contains(t, resp.Error, "device not found")
	// 1 initial + 2 retries = 3 total attempts
	assert.Equal(t, int32(3), atomic.LoadInt32(&attempts))
}

func TestBarkPusher_Push_RetryThenSuccess(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&attempts, 1)
		if count < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("service unavailable"))
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(barkResponse{
			Code:      200,
			Message:   "success",
			Timestamp: 9999,
		})
	}))
	defer server.Close()

	p := &BarkPusher{
		serverURL:  server.URL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
		maxRetries: 3,
	}

	resp, err := p.Push(context.Background(), PushRequest{
		DeviceKey: "test-key",
		Title:     "Test",
		Body:      "Test",
	})

	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, "9999", resp.MessageID)
	assert.Equal(t, int32(3), atomic.LoadInt32(&attempts))
}

func TestBarkPusher_Push_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error"))
	}))
	defer server.Close()

	p := &BarkPusher{
		serverURL:  server.URL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
		maxRetries: 3,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	resp, err := p.Push(ctx, PushRequest{
		DeviceKey: "test-key",
		Title:     "Test",
		Body:      "Test",
	})

	require.NoError(t, err)
	assert.False(t, resp.Success)
	// First attempt should fail due to cancelled context
}

func TestBarkPusher_Push_EmptyIcon(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body barkRequestBody
		json.NewDecoder(r.Body).Decode(&body)
		// Icon should not be present when empty
		assert.Empty(t, body.Icon)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(barkResponse{Code: 200, Message: "success", Timestamp: 1})
	}))
	defer server.Close()

	p := NewBarkPusher(config.BarkConfig{
		ServerURL:  server.URL,
		Timeout:    5,
		MaxRetries: 0,
	})

	resp, err := p.Push(context.Background(), PushRequest{
		DeviceKey: "test-key",
		Title:     "Test",
		Body:      "Test",
		Icon:      "", // empty icon
	})

	require.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestBarkPusher_PushToChannel_MultipleDevices(t *testing.T) {
	var receivedKeys []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract device key from path
		key := r.URL.Path[1:] // remove leading /
		receivedKeys = append(receivedKeys, key)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(barkResponse{Code: 200, Message: "success", Timestamp: 1})
	}))
	defer server.Close()

	p := NewBarkPusher(config.BarkConfig{
		ServerURL:  server.URL,
		Timeout:    5,
		MaxRetries: 0,
	})

	channel := config.Channel{
		Name:       "test-channel",
		Sound:      "alarm.caf",
		Group:      "test-group",
		Icon:       "https://example.com/icon.png",
		Level:      "active",
		DeviceKeys: []string{"device1", "device2", "device3"},
	}

	results := p.PushToChannel(context.Background(), PushRequest{
		Title: "Multi Device Test",
		Body:  "Hello all devices",
	}, channel, "default-key")

	assert.Len(t, results, 3)
	for _, r := range results {
		assert.True(t, r.Success)
	}
	assert.Equal(t, []string{"device1", "device2", "device3"}, receivedKeys)
}

func TestBarkPusher_PushToChannel_FallbackToDefault(t *testing.T) {
	var receivedKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedKey = r.URL.Path[1:]
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(barkResponse{Code: 200, Message: "success", Timestamp: 1})
	}))
	defer server.Close()

	p := NewBarkPusher(config.BarkConfig{
		ServerURL:  server.URL,
		Timeout:    5,
		MaxRetries: 0,
	})

	channel := config.Channel{
		Name:       "no-devices",
		Sound:      "glass.caf",
		Group:      "default",
		Level:      "active",
		DeviceKeys: nil, // no device keys configured
	}

	results := p.PushToChannel(context.Background(), PushRequest{
		Title: "Fallback Test",
		Body:  "Should use default key",
	}, channel, "global-default-key")

	assert.Len(t, results, 1)
	assert.True(t, results[0].Success)
	assert.Equal(t, "global-default-key", receivedKey)
}

func TestBarkPusher_PushToChannel_SingleDeviceFailure(t *testing.T) {
	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&callCount, 1)
		if count == 2 {
			// Second device fails
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("error"))
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(barkResponse{Code: 200, Message: "success", Timestamp: 1})
	}))
	defer server.Close()

	p := &BarkPusher{
		serverURL:  server.URL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
		maxRetries: 0, // no retries to speed up test
	}

	channel := config.Channel{
		Name:       "test",
		DeviceKeys: []string{"device1", "device2", "device3"},
	}

	results := p.PushToChannel(context.Background(), PushRequest{
		Title: "Test",
		Body:  "Test",
	}, channel, "")

	assert.Len(t, results, 3)
	assert.True(t, results[0].Success, "device1 should succeed")
	assert.False(t, results[1].Success, "device2 should fail")
	assert.True(t, results[2].Success, "device3 should succeed despite device2 failure")
}

func TestBarkPusher_PushToChannel_AppliesChannelParams(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body barkRequestBody
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "beacon.caf", body.Sound)
		assert.Equal(t, "monitoring", body.Group)
		assert.Equal(t, "https://example.com/monitor.png", body.Icon)
		assert.Equal(t, "timeSensitive", body.Level)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(barkResponse{Code: 200, Message: "success", Timestamp: 1})
	}))
	defer server.Close()

	p := NewBarkPusher(config.BarkConfig{
		ServerURL:  server.URL,
		Timeout:    5,
		MaxRetries: 0,
	})

	channel := config.Channel{
		Name:       "monitor",
		Sound:      "beacon.caf",
		Group:      "monitoring",
		Icon:       "https://example.com/monitor.png",
		Level:      "timeSensitive",
		DeviceKeys: []string{"device1"},
	}

	results := p.PushToChannel(context.Background(), PushRequest{
		Title: "Alert",
		Body:  "Server down",
		// No sound/group/icon/level set - should use channel defaults
	}, channel, "")

	assert.Len(t, results, 1)
	assert.True(t, results[0].Success)
}

func TestBarkPusher_Push_InvalidJSON_Response(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	p := &BarkPusher{
		serverURL:  server.URL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
		maxRetries: 0,
	}

	resp, err := p.Push(context.Background(), PushRequest{
		DeviceKey: "test-key",
		Title:     "Test",
		Body:      "Test",
	})

	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Error, "failed to parse response")
}
