package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestYAMLLoader_Load_Success(t *testing.T) {
	content := `
server:
  port: 8080
  api_key: "test-api-key-1234567890"
bark:
  server_url: "https://api.day.app"
  device_key: "test-device-key"
  timeout: 10
  max_retries: 3
channels:
  - name: urgent
    sound: "alarm.caf"
    group: "紧急"
    level: critical
rules:
  - name: github-ci-fail
    source: github
    content: "failed|error"
    channel: urgent
dnd:
  - start: "23:00"
    end: "07:30"
    days: everyday
templates:
  - name: github_alert
    content: "🚨 {{.source}}: {{.title}}"
aggregator:
  dedupe_window: 300
  batch_threshold: 10
  rate_window: 3600
  rate_limit: 30
  max_pending: 100
escalation:
  enabled: true
  wait_minutes: 5
  max_escalations: 3
  interval: increasing
  interval_minutes: 5
`
	tmpFile := writeTemp(t, content)
	loader := NewYAMLLoader()
	cfg, err := loader.Load(tmpFile)

	require.NoError(t, err)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, "test-api-key-1234567890", cfg.Server.APIKey)
	assert.Equal(t, "https://api.day.app", cfg.Bark.ServerURL)
	assert.Equal(t, "test-device-key", cfg.Bark.DeviceKey)
	assert.Equal(t, 10, cfg.Bark.Timeout)
	assert.Equal(t, 3, cfg.Bark.MaxRetries)
	assert.Len(t, cfg.Channels, 1)
	assert.Equal(t, "urgent", cfg.Channels[0].Name)
	assert.Equal(t, "alarm.caf", cfg.Channels[0].Sound)
	assert.Equal(t, "critical", cfg.Channels[0].Level)
	assert.Len(t, cfg.Rules, 1)
	assert.Equal(t, "github-ci-fail", cfg.Rules[0].Name)
	assert.Len(t, cfg.DND, 1)
	assert.Equal(t, "23:00", cfg.DND[0].Start)
	assert.Len(t, cfg.Templates, 1)
	assert.Equal(t, "github_alert", cfg.Templates[0].Name)
	assert.Equal(t, 300, cfg.Aggregator.DedupeWindow)
	assert.True(t, cfg.Escalation.Enabled)
	assert.Equal(t, 5, cfg.Escalation.WaitMinutes)
}

func TestYAMLLoader_Load_FileNotExist(t *testing.T) {
	loader := NewYAMLLoader()
	_, err := loader.Load("/nonexistent/path/config.yaml")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "配置文件不存在")
	assert.Contains(t, err.Error(), "/nonexistent/path/config.yaml")
}

func TestYAMLLoader_Load_PermissionDenied(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("skipping permission test in CI")
	}
	// Windows does not enforce POSIX file permissions via chmod
	if filepath.Separator == '\\' {
		t.Skip("skipping permission test on Windows")
	}

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "noperm.yaml")
	err := os.WriteFile(tmpFile, []byte("server:\n  port: 8080\n"), 0o000)
	require.NoError(t, err)
	t.Cleanup(func() { os.Chmod(tmpFile, 0o644) })

	loader := NewYAMLLoader()
	_, loadErr := loader.Load(tmpFile)

	require.Error(t, loadErr)
	assert.Contains(t, loadErr.Error(), "配置文件无读取权限")
	assert.Contains(t, loadErr.Error(), tmpFile)
}

func TestYAMLLoader_Load_YAMLSyntaxError(t *testing.T) {
	content := `
server:
  port: 8080
  api_key: "test"
  invalid_yaml: [unclosed
`
	tmpFile := writeTemp(t, content)
	loader := NewYAMLLoader()
	_, err := loader.Load(tmpFile)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "YAML 语法错误")
	assert.Contains(t, err.Error(), tmpFile)
}

func TestYAMLLoader_Load_EnvVarPath(t *testing.T) {
	content := `
server:
  port: 9090
`
	tmpFile := writeTemp(t, content)
	t.Setenv("OVERSEER_CONFIG_PATH", tmpFile)

	loader := NewYAMLLoader()
	cfg, err := loader.Load("")

	require.NoError(t, err)
	assert.Equal(t, 9090, cfg.Server.Port)
}

func TestYAMLLoader_Load_DefaultPath(t *testing.T) {
	// When no path and no env var, defaults to "config.yaml"
	t.Setenv("OVERSEER_CONFIG_PATH", "")

	loader := NewYAMLLoader()
	_, err := loader.Load("")

	// Should fail because config.yaml doesn't exist in test working dir
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config.yaml")
}

func TestResolvePath_Priority(t *testing.T) {
	t.Setenv("OVERSEER_CONFIG_PATH", "/env/path.yaml")

	// Explicit path takes priority
	assert.Equal(t, "/explicit/path.yaml", resolvePath("/explicit/path.yaml"))

	// Env var is used when path is empty
	assert.Equal(t, "/env/path.yaml", resolvePath(""))

	// Default when both are empty
	t.Setenv("OVERSEER_CONFIG_PATH", "")
	assert.Equal(t, "config.yaml", resolvePath(""))
}

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(tmpFile, []byte(content), 0o644)
	require.NoError(t, err)
	return tmpFile
}
