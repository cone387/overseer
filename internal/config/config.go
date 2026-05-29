package config

// Config is the root configuration structure for Overseer.
type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Bark       BarkConfig       `yaml:"bark"`
	Desktop    DesktopConfig    `yaml:"desktop"`
	LLM        LLMConfig        `yaml:"llm"`
	Channels   []Channel        `yaml:"channels"`
	Rules      []Rule           `yaml:"rules"`
	DND        []DNDPeriod      `yaml:"dnd"`
	Templates  []Template       `yaml:"templates"`
	Aggregator AggregatorConfig `yaml:"aggregator"`
	Escalation EscalationConfig `yaml:"escalation"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port      int    `yaml:"port"`
	APIKey    string `yaml:"api_key"`    // deprecated: kept for backward compat during migration
	JWTSecret string `yaml:"jwt_secret"` // auto-generated if empty
}

// LLMConfig holds LLM API settings for natural language schedule parsing.
type LLMConfig struct {
	BaseURL string `yaml:"base_url"` // OpenAI-compatible API base URL
	APIKey  string `yaml:"api_key"`  // API key
	Model   string `yaml:"model"`    // model name, e.g. "gpt-4o-mini"
}

// BarkConfig holds Bark push service settings.
type BarkConfig struct {
	ServerURL  string `yaml:"server_url"`
	DeviceKey  string `yaml:"device_key"`
	Timeout    int    `yaml:"timeout"`
	MaxRetries int    `yaml:"max_retries"`
}

// DesktopConfig holds desktop client registration settings.
type DesktopConfig struct {
	// RegisterToken is a shared secret required for desktop self-registration.
	// If empty, desktop registration is disabled.
	RegisterToken string `yaml:"register_token"`
	// MaxDevices limits the number of desktop devices that can be registered. 0 = unlimited.
	MaxDevices int `yaml:"max_devices"`
}

// Channel defines a notification channel with its push parameters.
type Channel struct {
	Name           string   `yaml:"name"`
	Sound          string   `yaml:"sound"`
	Group          string   `yaml:"group"`
	Icon           string   `yaml:"icon"`
	Level          string   `yaml:"level"`
	DeviceKeys     []string `yaml:"device_keys"`
	RequireAck     bool     `yaml:"require_ack"`
	RepeatInterval string   `yaml:"repeat_interval"` // e.g. "5m", "1h"
	MaxRepeats     int      `yaml:"max_repeats"`
	SoundFile      string   `yaml:"sound_file"`
}

// Rule defines a routing rule that maps messages to channels.
type Rule struct {
	Name     string `yaml:"name"`
	Source   string `yaml:"source"`
	Content  string `yaml:"content"`
	Channel  string `yaml:"channel"`
	Template string `yaml:"template"`
}

// DNDPeriod defines a Do Not Disturb time period.
type DNDPeriod struct {
	Start string `yaml:"start"`
	End   string `yaml:"end"`
	Days  string `yaml:"days"`
}

// Template defines a message template for rendering push content.
type Template struct {
	Name    string `yaml:"name"`
	Content string `yaml:"content"`
}

// AggregatorConfig holds message aggregation settings.
type AggregatorConfig struct {
	DedupeWindow   int `yaml:"dedupe_window"`
	BatchThreshold int `yaml:"batch_threshold"`
	RateWindow     int `yaml:"rate_window"`
	RateLimit      int `yaml:"rate_limit"`
	MaxPending     int `yaml:"max_pending"`
}

// EscalationConfig holds message escalation settings.
type EscalationConfig struct {
	Enabled         bool   `yaml:"enabled"`
	WaitMinutes     int    `yaml:"wait_minutes"`
	MaxEscalations  int    `yaml:"max_escalations"`
	Interval        string `yaml:"interval"`
	IntervalMinutes int    `yaml:"interval_minutes"`
}
