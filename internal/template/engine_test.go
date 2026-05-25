package template

import (
	"testing"
	"time"

	"github.com/overseer/overseer/internal/config"
	"github.com/overseer/overseer/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEngine_ValidTemplates(t *testing.T) {
	templates := []config.Template{
		{Name: "alert", Content: "🚨 {{.source}}: {{.title}}"},
		{Name: "event", Content: "📦 {{.title}}\n{{.body}}"},
	}

	engine, err := NewEngine(templates)
	require.NoError(t, err)
	assert.Len(t, engine.templates, 2)
}

func TestNewEngine_EmptyTemplates(t *testing.T) {
	engine, err := NewEngine(nil)
	require.NoError(t, err)
	assert.Empty(t, engine.templates)
}

func TestNewEngine_InvalidTemplateSyntax(t *testing.T) {
	templates := []config.Template{
		{Name: "bad", Content: "{{.unclosed"},
	}

	engine, err := NewEngine(templates)
	assert.Error(t, err)
	assert.Nil(t, engine)
	assert.Contains(t, err.Error(), "bad")
}

func TestRender_Success(t *testing.T) {
	templates := []config.Template{
		{Name: "alert", Content: "🚨 {{.source}}: {{.title}}"},
	}
	engine, err := NewEngine(templates)
	require.NoError(t, err)

	msg := &model.Message{
		Source:     "github",
		Title:      "CI Failed",
		Body:       "Build #123 failed",
		ReceivedAt: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
	}

	result, err := engine.Render("alert", msg)
	require.NoError(t, err)
	assert.Equal(t, "🚨 github: CI Failed", result)
}

func TestRender_AllFields(t *testing.T) {
	templates := []config.Template{
		{Name: "full", Content: "{{.source}}|{{.title}}|{{.body}}|{{.timestamp}}|{{index .extra \"key\"}}"},
	}
	engine, err := NewEngine(templates)
	require.NoError(t, err)

	msg := &model.Message{
		Source:     "grafana",
		Title:      "Alert",
		Body:       "CPU high",
		Extra:      map[string]string{"key": "value"},
		ReceivedAt: time.Date(2024, 6, 1, 14, 0, 0, 0, time.UTC),
	}

	result, err := engine.Render("full", msg)
	require.NoError(t, err)
	assert.Equal(t, "grafana|Alert|CPU high|2024-06-01 14:00:00|value", result)
}

func TestRender_TemplateNotFound_FallbackToBody(t *testing.T) {
	engine, err := NewEngine(nil)
	require.NoError(t, err)

	msg := &model.Message{
		Body: "original body content",
	}

	result, err := engine.Render("nonexistent", msg)
	require.NoError(t, err)
	assert.Equal(t, "original body content", result)
}

func TestRender_ExecutionError_FallbackToBody(t *testing.T) {
	// Use a template that calls a function on a value that will cause an execution error.
	// Calling "call" on a non-function value triggers an execution error.
	templates := []config.Template{
		{Name: "bad_exec", Content: "{{call .source}}"},
	}
	engine, err := NewEngine(templates)
	require.NoError(t, err)

	msg := &model.Message{
		Source:     "github",
		Body:       "fallback body",
		Extra:      nil,
		ReceivedAt: time.Now(),
	}

	result, err := engine.Render("bad_exec", msg)
	require.NoError(t, err)
	assert.Equal(t, "fallback body", result)
}

func TestRender_EmptyResult_FallbackToBody(t *testing.T) {
	// Template that renders to empty string using conditional
	templates := []config.Template{
		{Name: "empty", Content: "{{if eq .source \"never\"}}something{{end}}"},
	}
	engine, err := NewEngine(templates)
	require.NoError(t, err)

	msg := &model.Message{
		Source:     "github",
		Body:       "original content",
		ReceivedAt: time.Now(),
	}

	result, err := engine.Render("empty", msg)
	require.NoError(t, err)
	assert.Equal(t, "original content", result)
}

func TestRender_ExtraFieldAccess(t *testing.T) {
	templates := []config.Template{
		{Name: "extra_tmpl", Content: "repo: {{index .extra \"repo\"}} branch: {{index .extra \"branch\"}}"},
	}
	engine, err := NewEngine(templates)
	require.NoError(t, err)

	msg := &model.Message{
		Source:     "github",
		Title:      "Push",
		Body:       "New commit",
		Extra:      map[string]string{"repo": "overseer", "branch": "main"},
		ReceivedAt: time.Now(),
	}

	result, err := engine.Render("extra_tmpl", msg)
	require.NoError(t, err)
	assert.Equal(t, "repo: overseer branch: main", result)
}

func TestRender_TimestampFormat(t *testing.T) {
	templates := []config.Template{
		{Name: "ts", Content: "received: {{.timestamp}}"},
	}
	engine, err := NewEngine(templates)
	require.NoError(t, err)

	msg := &model.Message{
		Body:       "test",
		ReceivedAt: time.Date(2024, 12, 25, 8, 30, 45, 0, time.UTC),
	}

	result, err := engine.Render("ts", msg)
	require.NoError(t, err)
	assert.Equal(t, "received: 2024-12-25 08:30:45", result)
}

func TestRender_NilExtra(t *testing.T) {
	templates := []config.Template{
		{Name: "simple", Content: "{{.title}} - {{.body}}"},
	}
	engine, err := NewEngine(templates)
	require.NoError(t, err)

	msg := &model.Message{
		Title:      "Hello",
		Body:       "World",
		Extra:      nil,
		ReceivedAt: time.Now(),
	}

	result, err := engine.Render("simple", msg)
	require.NoError(t, err)
	assert.Equal(t, "Hello - World", result)
}
