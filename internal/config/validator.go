package config

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// validateConfig performs full validation of the Config structure.
// It returns a list of all validation errors found.
func validateConfig(cfg *Config) error {
	var errs []string

	errs = append(errs, validateRequiredFields(cfg)...)
	errs = append(errs, validateFieldConstraints(cfg)...)
	errs = append(errs, validateChannelUniqueness(cfg)...)
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
	if cfg.Server.APIKey == "" {
		errs = append(errs, "  - 缺少必填字段: server.api_key")
	}
	if cfg.Bark.ServerURL == "" {
		errs = append(errs, "  - 缺少必填字段: bark.server_url")
	}
	if cfg.Bark.DeviceKey == "" {
		errs = append(errs, "  - 缺少必填字段: bark.device_key")
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

	// server.api_key: length >= 16
	if cfg.Server.APIKey != "" && len(cfg.Server.APIKey) < 16 {
		errs = append(errs, fmt.Sprintf("  - 字段 server.api_key 长度为 %d，不满足最小长度要求 16", len(cfg.Server.APIKey)))
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
// If a channel has no device_keys configured, the global bark.device_key must be set.
func validateDeviceKeyAvailability(cfg *Config) []string {
	var errs []string

	for _, ch := range cfg.Channels {
		if len(ch.DeviceKeys) == 0 && cfg.Bark.DeviceKey == "" {
			errs = append(errs, fmt.Sprintf("  - Channel %q 未配置 device_keys 且全局 bark.device_key 也未配置", ch.Name))
		}
	}

	return errs
}

// isValidURL checks whether s is a valid URL with a scheme and host.
func isValidURL(s string) bool {
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	return u.Scheme != "" && u.Host != ""
}
