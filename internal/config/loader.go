package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"gopkg.in/yaml.v3"
)

// Loader defines the interface for loading and validating configuration.
type Loader interface {
	Load(path string) (*Config, error)
	Validate(cfg *Config) error
}

// YAMLLoader implements Loader for YAML configuration files.
type YAMLLoader struct{}

// NewYAMLLoader creates a new YAMLLoader instance.
func NewYAMLLoader() *YAMLLoader {
	return &YAMLLoader{}
}

// Load reads and parses a YAML configuration file.
// Resolution order for the config path:
//  1. If path is non-empty, use it directly.
//  2. If OVERSEER_CONFIG_PATH env var is set, use it.
//  3. Default to "config.yaml" in the current directory.
func (l *YAMLLoader) Load(path string) (*Config, error) {
	resolvedPath := resolvePath(path)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return nil, formatFileError(resolvedPath, err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, formatYAMLError(resolvedPath, err)
	}

	return cfg, nil
}

// Validate checks the configuration for correctness.
// It validates required fields, field constraints, channel uniqueness,
// rule regex syntax, and device_key availability.
func (l *YAMLLoader) Validate(cfg *Config) error {
	return validateConfig(cfg)
}

// resolvePath determines the configuration file path based on the priority:
// explicit path > env var > default.
func resolvePath(path string) string {
	if path != "" {
		return path
	}
	if envPath := os.Getenv("OVERSEER_CONFIG_PATH"); envPath != "" {
		return envPath
	}
	return "config.yaml"
}

// formatFileError produces a clear error message for file access failures.
func formatFileError(path string, err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("配置文件不存在: %s", path)
	}
	if errors.Is(err, fs.ErrPermission) {
		return fmt.Errorf("配置文件无读取权限: %s", path)
	}
	return fmt.Errorf("读取配置文件失败: %s: %w", path, err)
}

// formatYAMLError produces a clear error message for YAML parsing failures,
// including line number when available.
func formatYAMLError(path string, err error) error {
	var typeErr *yaml.TypeError
	if errors.As(err, &typeErr) {
		return fmt.Errorf("配置文件 %s YAML 类型错误: %s", path, typeErr.Errors[0])
	}

	// yaml.v3 embeds line info in the error message for syntax errors.
	return fmt.Errorf("配置文件 %s YAML 语法错误: %v", path, err)
}
