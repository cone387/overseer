package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Device represents the response from the desktop registration endpoint.
type Device struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	DeviceKey string `json:"device_key"`
	Type      string `json:"type"`
	IsDefault bool   `json:"is_default"`
}

// registerRequest is the request body for desktop registration.
type registerRequest struct {
	Name  string `json:"name"`
	Token string `json:"token"`
}

// apiResponse wraps the standard Overseer API response format.
type apiResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

var httpClient = &http.Client{
	Timeout: 10 * time.Second,
}

// Register calls POST /api/devices/desktop-register to register this desktop client.
func Register(baseURL, name, token string) (*Device, error) {
	body, err := json.Marshal(registerRequest{
		Name:  name,
		Token: token,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := baseURL + "/api/devices/desktop-register"
	resp, err := httpClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("parse response: %w (body: %s)", err, string(respBody))
	}

	if apiResp.Code != 200 {
		return nil, fmt.Errorf("registration failed: %s (code %d)", apiResp.Message, apiResp.Code)
	}

	var device Device
	if err := json.Unmarshal(apiResp.Data, &device); err != nil {
		return nil, fmt.Errorf("parse device data: %w", err)
	}

	return &device, nil
}
