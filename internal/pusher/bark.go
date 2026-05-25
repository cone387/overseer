package pusher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/overseer/overseer/internal/config"
)

// BarkPusher implements the Pusher interface for the Bark push service.
type BarkPusher struct {
	serverURL  string
	httpClient *http.Client
	maxRetries int
}

// barkRequestBody is the JSON payload sent to the Bark server.
type barkRequestBody struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Sound string `json:"sound,omitempty"`
	Group string `json:"group,omitempty"`
	Icon  string `json:"icon,omitempty"`
	Level string `json:"level,omitempty"`
}

// barkResponse is the JSON response from the Bark server.
type barkResponse struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

// NewBarkPusher creates a new BarkPusher with the given configuration.
func NewBarkPusher(cfg config.BarkConfig) *BarkPusher {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10
	}
	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}
	return &BarkPusher{
		serverURL: cfg.ServerURL,
		httpClient: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
		maxRetries: maxRetries,
	}
}

// Name returns the name of this pusher.
func (b *BarkPusher) Name() string {
	return "bark"
}

// Push sends a push notification to a single device via the Bark server.
// It implements exponential backoff retry on failure (1s, 2s, 4s).
func (b *BarkPusher) Push(ctx context.Context, req PushRequest) (*PushResponse, error) {
	if req.DeviceKey == "" {
		return &PushResponse{
			Success: false,
			Error:   "device_key is empty",
		}, nil
	}

	body := barkRequestBody{
		Title: req.Title,
		Body:  req.Body,
		Sound: req.Sound,
		Group: req.Group,
		Level: req.Level,
	}
	if req.Icon != "" {
		body.Icon = req.Icon
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return &PushResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to marshal request body: %v", err),
		}, nil
	}

	url := fmt.Sprintf("%s/%s", b.serverURL, req.DeviceKey)

	var lastErr string
	backoff := time.Second // initial backoff: 1s

	for attempt := 0; attempt <= b.maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return &PushResponse{
					Success: false,
					Error:   fmt.Sprintf("context cancelled during retry: %v", ctx.Err()),
				}, nil
			case <-time.After(backoff):
				backoff *= 2 // exponential: 1s -> 2s -> 4s
			}
		}

		resp, err := b.doRequest(ctx, url, payload)
		if err != nil {
			lastErr = err.Error()
			continue
		}

		if resp.Success {
			return resp, nil
		}
		lastErr = resp.Error
	}

	return &PushResponse{
		Success: false,
		Error:   lastErr,
	}, nil
}

// doRequest performs a single HTTP POST request to the Bark server.
func (b *BarkPusher) doRequest(ctx context.Context, url string, payload []byte) (*PushResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := b.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return &PushResponse{
			Success: false,
			Error:   fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(respBody)),
		}, nil
	}

	var barkResp barkResponse
	if err := json.Unmarshal(respBody, &barkResp); err != nil {
		return &PushResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to parse response: %v, body: %s", err, string(respBody)),
		}, nil
	}

	if barkResp.Code != 200 {
		return &PushResponse{
			Success: false,
			Error:   fmt.Sprintf("bark error code %d: %s", barkResp.Code, barkResp.Message),
		}, nil
	}

	return &PushResponse{
		Success:   true,
		MessageID: fmt.Sprintf("%d", barkResp.Timestamp),
	}, nil
}

// PushToChannel sends a push notification to all devices configured for a channel.
// If the channel has no device_keys configured, it falls back to the global default device_key.
// Single device failure does not affect other devices.
func (b *BarkPusher) PushToChannel(ctx context.Context, req PushRequest, channel config.Channel, defaultDeviceKey string) []*PushResponse {
	deviceKeys := channel.DeviceKeys
	if len(deviceKeys) == 0 {
		deviceKeys = []string{defaultDeviceKey}
	}

	results := make([]*PushResponse, 0, len(deviceKeys))
	for _, dk := range deviceKeys {
		deviceReq := req
		deviceReq.DeviceKey = dk
		// Apply channel parameters
		if deviceReq.Sound == "" {
			deviceReq.Sound = channel.Sound
		}
		if deviceReq.Group == "" {
			deviceReq.Group = channel.Group
		}
		if deviceReq.Icon == "" {
			deviceReq.Icon = channel.Icon
		}
		if deviceReq.Level == "" {
			deviceReq.Level = channel.Level
		}

		resp, _ := b.Push(ctx, deviceReq)
		results = append(results, resp)
	}

	return results
}
