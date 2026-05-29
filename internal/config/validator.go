package config

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// validateConfig performs full validation of the Config structure.
// It returns a list of all validation errors found.
func validateConfig(cfg *Config) error {
	var errs []string

	errs = append(errs, validateRequiredFields(cfg)...)
	errs = append(errs, validateFieldConstraints(cfg)...)
	errs = append(errs, validateChannelUniqueness(cfg)...)
	errs = append(errs, validateChannelLifecycle(cfg)...)
	errs = append(errs, validateRuleRegex(cfg)...)
	errs = append(errs, validateDeviceKeyAvailability(cfg)...)

	if len(errs) > 0 {
		return fmt.Errorf("配置验证失败:\n%s", strings.Join(errs, "\n"))
	}
	return nil
}

// validateRequiredFields checks that all mandatory fields are present.
func validateRequiredFields(cfg *Config) []string {
	var errs []string

	if cfg.Server.Port == 0 {
		errs = append(errs, "  - 缺少必填字段: server.port")
	}
	if cfg.Bark.ServerURL == "" {
		errs = append(errs, "  - 缺少必填字段: bark.server_url")
	}

	return errs
}

// validateFieldConstraints checks value constraints on fields.
func validateFieldConstraints(cfg *Config) []string {
	var errs []string

	// server.port: 1-65535
	if cfg.Server.Port != 0 && (cfg.Server.Port < 1 || cfg.Server.Port > 65535) {
		errs = append(errs, fmt.Sprintf("  - 字段 server.port 值 %d 超出有效范围 1-65535", cfg.Server.Port))
	}

	// bark.server_url: valid URL
	if cfg.Bark.ServerURL != "" {
		if !isValidURL(cfg.Bark.ServerURL) {
			errs = append(errs, fmt.Sprintf("  - 字段 bark.server_url 值 %q 不是合法的 URL 格式", cfg.Bark.ServerURL))
		}
	}

	return errs
}

// validateChannelUniqueness checks that all channel names are unique.
func validateChannelUniqueness(cfg *Config) []string {
	var errs []string
	seen := make(map[string]bool)

	for _, ch := range cfg.Channels {
		if ch.Name == "" {
			continue
		}
		if seen[ch.Name] {
			errs = append(errs, fmt.Sprintf("  - Channel 名称重复: %q", ch.Name))
		}
		seen[ch.Name] = true
	}

	return errs
}

// validateRuleRegex checks that content regex patterns in rules are valid.
func validateRuleRegex(cfg *Config) []string {
	var errs []string

	for _, rule := range cfg.Rules {
		if rule.Content == "" {
			continue
		}
		if _, err := regexp.Compile(rule.Content); err != nil {
			errs = append(errs, fmt.Sprintf("  - 规则 %q 的 content 正则表达式语法无效: %s", rule.Name, err.Error()))
		}
	}

	return errs
}

// validateDeviceKeyAvailability checks that every channel has a usable device_key.
// validateDeviceKeyAvailability is no longer needed since devices are managed via the web UI.
func validateDeviceKeyAvailability(cfg *Config) []string {
	return nil
}

// isValidURL checks whether s is a valid URL with a scheme and host.
func isValidURL(s string) bool {
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	return u.Scheme != "" && u.Host != ""
}

// validateChannelLifecycle checks lifecycle configuration for channels with require_ack enabled.
func validateChannelLifecycle(cfg *Config) []string {
	var errs []string

	for _, ch := range cfg.Channels {
		if !ch.RequireAck {
			continue
		}

		// repeat_interval must be set and valid when require_ack is true
		if ch.RepeatInterval == "" {
			errs = append(errs, fmt.Sprintf("  - Channel %q: require_ack 为 true 时必须设置 repeat_interval", ch.Name))
		} else {
			d, err := time.ParseDuration(ch.RepeatInterval)
			if err != nil {
				errs = append(errs, fmt.Sprintf("  - Channel %q: repeat_interval %q 无法解析为有效的时间间隔: %s", ch.Name, ch.RepeatInterval, err.Error()))
			} else if d < time.Minute || d > 24*time.Hour {
				errs = append(errs, fmt.Sprintf("  - Channel %q: repeat_interval %q 超出有效范围 1m-24h", ch.Name, ch.RepeatInterval))
			}
		}

		// max_repeats must be between 1 and 100 when require_ack is true
		if ch.MaxRepeats < 1 || ch.MaxRepeats > 100 {
			errs = append(errs, fmt.Sprintf("  - Channel %q: max_repeats 值 %d 超出有效范围 1-100", ch.Name, ch.MaxRepeats))
		}
	}

	return errs
}

// ValidateTTL validates a TTL string parses to a duration between 1 minute and 30 days.
// Returns the parsed duration on success, or an error if the TTL is invalid.
func ValidateTTL(ttl string) (time.Duration, error) {
	if ttl == "" {
		return 0, fmt.Errorf("ttl 不能为空")
	}

	d, err := time.ParseDuration(ttl)
	if err != nil {
		return 0, fmt.Errorf("ttl %q 无法解析为有效的时间间隔: %s", ttl, err.Error())
	}

	if d < time.Minute {
		return 0, fmt.Errorf("ttl %q 小于最小值 1m", ttl)
	}

	if d > 30*24*time.Hour {
		return 0, fmt.Errorf("ttl %q 超过最大值 30 天 (720h)", ttl)
	}

	return d, nil
}
