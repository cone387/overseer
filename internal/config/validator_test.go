package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validConfig returns a minimal valid Config for testing.
func validConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port: 8080,
		},
		Bark: BarkConfig{
			ServerURL: "https://api.day.app",
			DeviceKey: "test-device-key",
		},
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	loader := NewYAMLLoader()
	cfg := validConfig()

	err := loader.Validate(cfg)
	assert.NoError(t, err)
}

// --- Required fields ---

func TestValidate_MissingServerPort(t *testing.T) {
	cfg := validConfig()
	cfg.Server.Port = 0

	err := validateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server.port")
}

func TestValidate_MissingServerURL(t *testing.T) {
	cfg := validConfig()
	cfg.Bark.ServerURL = ""

	err := validateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bark.server_url")
}

func TestValidate_MultipleRequiredFieldsMissing(t *testing.T) {
	cfg := &Config{}

	err := validateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server.port")
	assert.Contains(t, err.Error(), "bark.server_url")
}

// --- Field constraints ---

func TestValidate_PortTooLow(t *testing.T) {
	cfg := validConfig()
	cfg.Server.Port = -1

	err := validateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server.port")
	assert.Contains(t, err.Error(), "1-65535")
}

func TestValidate_PortTooHigh(t *testing.T) {
	cfg := validConfig()
	cfg.Server.Port = 65536

	err := validateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server.port")
	assert.Contains(t, err.Error(), "1-65535")
}

func TestValidate_PortBoundaryValid(t *testing.T) {
	cfg := validConfig()

	cfg.Server.Port = 1
	assert.NoError(t, validateConfig(cfg))

	cfg.Server.Port = 65535
	assert.NoError(t, validateConfig(cfg))
}

func TestValidate_ServerURLInvalid(t *testing.T) {
	cfg := validConfig()
	cfg.Bark.ServerURL = "not-a-url"

	err := validateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bark.server_url")
	assert.Contains(t, err.Error(), "URL")
}

func TestValidate_ServerURLNoScheme(t *testing.T) {
	cfg := validConfig()
	cfg.Bark.ServerURL = "api.day.app"

	err := validateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bark.server_url")
}

func TestValidate_ServerURLValid(t *testing.T) {
	cfg := validConfig()

	cfg.Bark.ServerURL = "https://api.day.app"
	assert.NoError(t, validateConfig(cfg))

	cfg.Bark.ServerURL = "http://localhost:8080"
	assert.NoError(t, validateConfig(cfg))

	cfg.Bark.ServerURL = "https://192.168.1.1:443/push"
	assert.NoError(t, validateConfig(cfg))
}

// --- Channel name uniqueness ---

func TestValidate_DuplicateChannelNames(t *testing.T) {
	cfg := validConfig()
	cfg.Channels = []Channel{
		{Name: "urgent"},
		{Name: "default"},
		{Name: "urgent"},
	}

	err := validateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Channel 名称重复")
	assert.Contains(t, err.Error(), "urgent")
}

func TestValidate_UniqueChannelNames(t *testing.T) {
	cfg := validConfig()
	cfg.Channels = []Channel{
		{Name: "urgent"},
		{Name: "default"},
		{Name: "github"},
	}

	err := validateConfig(cfg)
	assert.NoError(t, err)
}

// --- Rule regex validation ---

func TestValidate_InvalidRegex(t *testing.T) {
	cfg := validConfig()
	cfg.Rules = []Rule{
		{Name: "bad-rule", Content: "[unclosed", Channel: "default"},
	}

	err := validateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad-rule")
	assert.Contains(t, err.Error(), "正则表达式语法无效")
}

func TestValidate_ValidRegex(t *testing.T) {
	cfg := validConfig()
	cfg.Rules = []Rule{
		{Name: "good-rule", Content: "failed|error", Channel: "default"},
		{Name: "complex-rule", Content: `^\[ALERT\]\s+\w+`, Channel: "urgent"},
	}

	err := validateConfig(cfg)
	assert.NoError(t, err)
}

func TestValidate_EmptyContentRegexSkipped(t *testing.T) {
	cfg := validConfig()
	cfg.Rules = []Rule{
		{Name: "source-only", Source: "github", Channel: "default"},
	}

	err := validateConfig(cfg)
	assert.NoError(t, err)
}

// --- Device key availability (now managed via web UI, no config validation) ---

func TestValidate_ChannelWithoutDeviceKeysNoError(t *testing.T) {
	cfg := validConfig()
	cfg.Bark.DeviceKey = ""
	cfg.Channels = []Channel{
		{Name: "urgent"},
	}

	// No longer an error - devices are managed via web UI
	err := validateConfig(cfg)
	assert.NoError(t, err)
}

// --- Integration: multiple errors reported together ---

func TestValidate_MultipleErrors(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			Port: 99999,
		},
		Bark: BarkConfig{
			ServerURL: "invalid",
			DeviceKey: "key",
		},
		Channels: []Channel{
			{Name: "ch1"},
			{Name: "ch1"},
		},
		Rules: []Rule{
			{Name: "bad", Content: "(unclosed", Channel: "ch1"},
		},
	}

	err := validateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server.port")
	assert.Contains(t, err.Error(), "bark.server_url")
	assert.Contains(t, err.Error(), "Channel 名称重复")
	assert.Contains(t, err.Error(), "正则表达式语法无效")
}
