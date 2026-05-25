package router

import (
	"testing"

	"github.com/overseer/overseer/internal/config"
	"github.com/overseer/overseer/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRouter_ValidRules(t *testing.T) {
	rules := []config.Rule{
		{Name: "r1", Source: "github", Content: "failed|error", Channel: "urgent"},
		{Name: "r2", Source: "grafana", Channel: "monitor"},
	}
	channels := []config.Channel{
		{Name: "urgent", Sound: "alarm.caf", Group: "紧急", Level: "critical"},
		{Name: "monitor", Sound: "beacon.caf", Group: "监控", Level: "timeSensitive"},
		{Name: "default", Sound: "", Group: "default", Level: "active"},
	}

	r, err := NewRouter(rules, channels)
	require.NoError(t, err)
	assert.NotNil(t, r)
	assert.Len(t, r.rules, 2)
}

func TestNewRouter_InvalidRegex(t *testing.T) {
	rules := []config.Rule{
		{Name: "bad-rule", Content: "[invalid", Channel: "default"},
	}
	channels := []config.Channel{
		{Name: "default", Sound: "", Group: "default", Level: "active"},
	}

	_, err := NewRouter(rules, channels)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad-rule")
	assert.Contains(t, err.Error(), "invalid content regex")
}

func TestRoute_SourceMatch(t *testing.T) {
	rules := []config.Rule{
		{Name: "github-events", Source: "github", Channel: "github"},
	}
	channels := []config.Channel{
		{Name: "github", Sound: "glass.caf", Group: "开发", Level: "active"},
		{Name: "default", Sound: "", Group: "default", Level: "active"},
	}

	r, err := NewRouter(rules, channels)
	require.NoError(t, err)

	msg := &model.Message{Source: "github", Title: "PR merged", Body: "PR #42 merged"}
	ch, tmpl := r.Route(msg)
	assert.Equal(t, "github", ch.Name)
	assert.Equal(t, "", tmpl)
}

func TestRoute_ContentRegexMatch(t *testing.T) {
	rules := []config.Rule{
		{Name: "error-detect", Content: "error|failed", Channel: "urgent"},
	}
	channels := []config.Channel{
		{Name: "urgent", Sound: "alarm.caf", Group: "紧急", Level: "critical"},
		{Name: "default", Sound: "", Group: "default", Level: "active"},
	}

	r, err := NewRouter(rules, channels)
	require.NoError(t, err)

	msg := &model.Message{Source: "ci", Title: "Build", Body: "Build failed on main"}
	ch, _ := r.Route(msg)
	assert.Equal(t, "urgent", ch.Name)

	// Non-matching body
	msg2 := &model.Message{Source: "ci", Title: "Build", Body: "Build succeeded"}
	ch2, _ := r.Route(msg2)
	assert.Equal(t, "default", ch2.Name)
}

func TestRoute_SourceAndContentANDLogic(t *testing.T) {
	rules := []config.Rule{
		{Name: "github-ci-fail", Source: "github", Content: "failed|error", Channel: "urgent", Template: "github_alert"},
		{Name: "github-events", Source: "github", Channel: "github", Template: "github_event"},
	}
	channels := []config.Channel{
		{Name: "urgent", Sound: "alarm.caf", Group: "紧急", Level: "critical"},
		{Name: "github", Sound: "glass.caf", Group: "开发", Level: "active"},
		{Name: "default", Sound: "", Group: "default", Level: "active"},
	}

	r, err := NewRouter(rules, channels)
	require.NoError(t, err)

	// Source matches "github" AND body matches "failed" → urgent
	msg := &model.Message{Source: "github", Title: "CI", Body: "CI failed"}
	ch, tmpl := r.Route(msg)
	assert.Equal(t, "urgent", ch.Name)
	assert.Equal(t, "github_alert", tmpl)

	// Source matches "github" but body doesn't match "failed|error" → falls through to second rule
	msg2 := &model.Message{Source: "github", Title: "PR", Body: "PR opened"}
	ch2, tmpl2 := r.Route(msg2)
	assert.Equal(t, "github", ch2.Name)
	assert.Equal(t, "github_event", tmpl2)
}

func TestRoute_FirstMatchWins(t *testing.T) {
	rules := []config.Rule{
		{Name: "rule-a", Source: "src", Channel: "chan-a"},
		{Name: "rule-b", Source: "src", Channel: "chan-b"},
	}
	channels := []config.Channel{
		{Name: "chan-a", Sound: "a.caf", Group: "A", Level: "active"},
		{Name: "chan-b", Sound: "b.caf", Group: "B", Level: "active"},
		{Name: "default", Sound: "", Group: "default", Level: "active"},
	}

	r, err := NewRouter(rules, channels)
	require.NoError(t, err)

	msg := &model.Message{Source: "src", Body: "anything"}
	ch, _ := r.Route(msg)
	assert.Equal(t, "chan-a", ch.Name)
}

func TestRoute_FallbackToDefault(t *testing.T) {
	rules := []config.Rule{
		{Name: "specific", Source: "github", Channel: "github"},
	}
	channels := []config.Channel{
		{Name: "github", Sound: "glass.caf", Group: "开发", Level: "active"},
		{Name: "default", Sound: "", Group: "default", Level: "active"},
	}

	r, err := NewRouter(rules, channels)
	require.NoError(t, err)

	// Message from unknown source → no rule matches → default
	msg := &model.Message{Source: "unknown", Title: "Hello", Body: "World"}
	ch, tmpl := r.Route(msg)
	assert.Equal(t, "default", ch.Name)
	assert.Equal(t, "", tmpl)
}

func TestRoute_FallbackWhenNoDefaultChannelConfigured(t *testing.T) {
	rules := []config.Rule{
		{Name: "specific", Source: "github", Channel: "github"},
	}
	channels := []config.Channel{
		{Name: "github", Sound: "glass.caf", Group: "开发", Level: "active"},
		// No "default" channel defined
	}

	r, err := NewRouter(rules, channels)
	require.NoError(t, err)

	msg := &model.Message{Source: "unknown", Title: "Hello", Body: "World"}
	ch, _ := r.Route(msg)
	// Should use built-in default fallback
	assert.Equal(t, "default", ch.Name)
	assert.Equal(t, "", ch.Sound)
	assert.Equal(t, "default", ch.Group)
	assert.Equal(t, "active", ch.Level)
}

func TestRoute_RuleTargetsNonexistentChannel(t *testing.T) {
	rules := []config.Rule{
		{Name: "bad-channel", Source: "src", Channel: "nonexistent"},
	}
	channels := []config.Channel{
		{Name: "default", Sound: "", Group: "default", Level: "active"},
	}

	r, err := NewRouter(rules, channels)
	require.NoError(t, err)

	msg := &model.Message{Source: "src", Body: "test"}
	ch, _ := r.Route(msg)
	// Rule matches but target channel doesn't exist → fallback to default
	assert.Equal(t, "default", ch.Name)
}

func TestRoute_EmptyRules(t *testing.T) {
	channels := []config.Channel{
		{Name: "default", Sound: "", Group: "default", Level: "active"},
	}

	r, err := NewRouter(nil, channels)
	require.NoError(t, err)

	msg := &model.Message{Source: "any", Title: "Test", Body: "Body"}
	ch, tmpl := r.Route(msg)
	assert.Equal(t, "default", ch.Name)
	assert.Equal(t, "", tmpl)
}

func TestRoute_RuleWithNoConditionsMatchesAll(t *testing.T) {
	rules := []config.Rule{
		{Name: "catch-all", Channel: "catch"},
	}
	channels := []config.Channel{
		{Name: "catch", Sound: "catch.caf", Group: "all", Level: "active"},
		{Name: "default", Sound: "", Group: "default", Level: "active"},
	}

	r, err := NewRouter(rules, channels)
	require.NoError(t, err)

	msg := &model.Message{Source: "anything", Title: "Any", Body: "Any body"}
	ch, _ := r.Route(msg)
	assert.Equal(t, "catch", ch.Name)
}

func TestRoute_ContentRegexCaseInsensitive(t *testing.T) {
	rules := []config.Rule{
		{Name: "case-test", Content: "(?i)error", Channel: "urgent"},
	}
	channels := []config.Channel{
		{Name: "urgent", Sound: "alarm.caf", Group: "紧急", Level: "critical"},
		{Name: "default", Sound: "", Group: "default", Level: "active"},
	}

	r, err := NewRouter(rules, channels)
	require.NoError(t, err)

	msg := &model.Message{Source: "ci", Body: "ERROR: something went wrong"}
	ch, _ := r.Route(msg)
	assert.Equal(t, "urgent", ch.Name)
}
