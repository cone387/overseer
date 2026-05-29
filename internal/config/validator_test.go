package config

import (
	"testing"
	"time"

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

// --- Channel lifecycle validation ---

func TestValidate_RequireAckWithValidConfig(t *testing.T) {
	cfg := validConfig()
	cfg.Channels = []Channel{
		{Name: "alerts", RequireAck: true, RepeatInterval: "5m", MaxRepeats: 10},
	}

	err := validateConfig(cfg)
	assert.NoError(t, err)
}

func TestValidate_RequireAckMissingRepeatInterval(t *testing.T) {
	cfg := validConfig()
	cfg.Channels = []Channel{
		{Name: "alerts", RequireAck: true, RepeatInterval: "", MaxRepeats: 10},
	}

	err := validateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "alerts")
	assert.Contains(t, err.Error(), "repeat_interval")
}

func TestValidate_RequireAckInvalidRepeatInterval(t *testing.T) {
	cfg := validConfig()
	cfg.Channels = []Channel{
		{Name: "alerts", RequireAck: true, RepeatInterval: "not-a-duration", MaxRepeats: 10},
	}

	err := validateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "alerts")
	assert.Contains(t, err.Error(), "repeat_interval")
	assert.Contains(t, err.Error(), "无法解析")
}

func TestValidate_RequireAckRepeatIntervalTooShort(t *testing.T) {
	cfg := validConfig()
	cfg.Channels = []Channel{
		{Name: "alerts", RequireAck: true, RepeatInterval: "30s", MaxRepeats: 10},
	}

	err := validateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "alerts")
	assert.Contains(t, err.Error(), "repeat_interval")
	assert.Contains(t, err.Error(), "1m-24h")
}

func TestValidate_RequireAckRepeatIntervalTooLong(t *testing.T) {
	cfg := validConfig()
	cfg.Channels = []Channel{
		{Name: "alerts", RequireAck: true, RepeatInterval: "25h", MaxRepeats: 10},
	}

	err := validateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "alerts")
	assert.Contains(t, err.Error(), "repeat_interval")
	assert.Contains(t, err.Error(), "1m-24h")
}

func TestValidate_RequireAckRepeatIntervalBoundary(t *testing.T) {
	cfg := validConfig()

	// Exactly 1 minute - valid
	cfg.Channels = []Channel{
		{Name: "alerts", RequireAck: true, RepeatInterval: "1m", MaxRepeats: 5},
	}
	assert.NoError(t, validateConfig(cfg))

	// Exactly 24 hours - valid
	cfg.Channels = []Channel{
		{Name: "alerts", RequireAck: true, RepeatInterval: "24h", MaxRepeats: 5},
	}
	assert.NoError(t, validateConfig(cfg))
}

func TestValidate_RequireAckMaxRepeatsTooLow(t *testing.T) {
	cfg := validConfig()
	cfg.Channels = []Channel{
		{Name: "alerts", RequireAck: true, RepeatInterval: "5m", MaxRepeats: 0},
	}

	err := validateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "alerts")
	assert.Contains(t, err.Error(), "max_repeats")
	assert.Contains(t, err.Error(), "1-100")
}

func TestValidate_RequireAckMaxRepeatsTooHigh(t *testing.T) {
	cfg := validConfig()
	cfg.Channels = []Channel{
		{Name: "alerts", RequireAck: true, RepeatInterval: "5m", MaxRepeats: 101},
	}

	err := validateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "alerts")
	assert.Contains(t, err.Error(), "max_repeats")
	assert.Contains(t, err.Error(), "1-100")
}

func TestValidate_RequireAckMaxRepeatsBoundary(t *testing.T) {
	cfg := validConfig()

	// Exactly 1 - valid
	cfg.Channels = []Channel{
		{Name: "alerts", RequireAck: true, RepeatInterval: "5m", MaxRepeats: 1},
	}
	assert.NoError(t, validateConfig(cfg))

	// Exactly 100 - valid
	cfg.Channels = []Channel{
		{Name: "alerts", RequireAck: true, RepeatInterval: "5m", MaxRepeats: 100},
	}
	assert.NoError(t, validateConfig(cfg))
}

func TestValidate_RequireAckFalseSkipsValidation(t *testing.T) {
	cfg := validConfig()
	// When require_ack is false, repeat_interval and max_repeats are ignored
	cfg.Channels = []Channel{
		{Name: "info", RequireAck: false, RepeatInterval: "", MaxRepeats: 0},
	}

	err := validateConfig(cfg)
	assert.NoError(t, err)
}

func TestValidate_RequireAckFalseWithInvalidValuesNoError(t *testing.T) {
	cfg := validConfig()
	// Even with invalid values, no error when require_ack is false
	cfg.Channels = []Channel{
		{Name: "info", RequireAck: false, RepeatInterval: "invalid", MaxRepeats: 999},
	}

	err := validateConfig(cfg)
	assert.NoError(t, err)
}

// --- ValidateTTL ---

func TestValidateTTL_Valid(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
	}{
		{"1m", time.Minute},
		{"5m", 5 * time.Minute},
		{"1h", time.Hour},
		{"24h", 24 * time.Hour},
		{"720h", 720 * time.Hour}, // 30 days
		{"168h", 168 * time.Hour}, // 7 days
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			d, err := ValidateTTL(tt.input)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, d)
		})
	}
}

func TestValidateTTL_Empty(t *testing.T) {
	_, err := ValidateTTL("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "不能为空")
}

func TestValidateTTL_InvalidFormat(t *testing.T) {
	_, err := ValidateTTL("not-a-duration")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无法解析")
}

func TestValidateTTL_TooShort(t *testing.T) {
	_, err := ValidateTTL("30s")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "小于最小值")
}

func TestValidateTTL_TooLong(t *testing.T) {
	_, err := ValidateTTL("721h") // > 30 days
	require.Error(t, err)
	assert.Contains(t, err.Error(), "超过最大值")
}

func TestValidateTTL_Boundary(t *testing.T) {
	// Exactly 1 minute - valid
	d, err := ValidateTTL("1m")
	assert.NoError(t, err)
	assert.Equal(t, time.Minute, d)

	// Exactly 30 days (720h) - valid
	d, err = ValidateTTL("720h")
	assert.NoError(t, err)
	assert.Equal(t, 720*time.Hour, d)
}
