package channel

import (
	"testing"

	"github.com/overseer/overseer/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestGetChannel_ExistingChannel(t *testing.T) {
	channels := []config.Channel{
		{Name: "urgent", Sound: "alarm.caf", Group: "紧急", Level: "critical"},
		{Name: "github", Sound: "glass.caf", Group: "开发", Level: "active"},
		{Name: "default", Sound: "default.caf", Group: "默认", Level: "active"},
	}

	mgr := NewManager(channels)

	ch := mgr.GetChannel("urgent")
	assert.Equal(t, "urgent", ch.Name)
	assert.Equal(t, "alarm.caf", ch.Sound)
	assert.Equal(t, "紧急", ch.Group)
	assert.Equal(t, "critical", ch.Level)

	ch = mgr.GetChannel("github")
	assert.Equal(t, "github", ch.Name)
	assert.Equal(t, "glass.caf", ch.Sound)
}

func TestGetChannel_NonExistentFallsBackToDefault(t *testing.T) {
	channels := []config.Channel{
		{Name: "urgent", Sound: "alarm.caf", Group: "紧急", Level: "critical"},
		{Name: "default", Sound: "ping.caf", Group: "通用", Level: "active"},
	}

	mgr := NewManager(channels)

	ch := mgr.GetChannel("nonexistent")
	assert.Equal(t, "default", ch.Name)
	assert.Equal(t, "ping.caf", ch.Sound)
	assert.Equal(t, "通用", ch.Group)
	assert.Equal(t, "active", ch.Level)
}

func TestGetChannel_NoDefaultUsesBuiltinDefault(t *testing.T) {
	channels := []config.Channel{
		{Name: "urgent", Sound: "alarm.caf", Group: "紧急", Level: "critical"},
		{Name: "github", Sound: "glass.caf", Group: "开发", Level: "active"},
	}

	mgr := NewManager(channels)

	// Request a non-existent channel; no "default" configured → built-in default.
	ch := mgr.GetChannel("nonexistent")
	assert.Equal(t, "default", ch.Name)
	assert.Equal(t, "", ch.Sound)
	assert.Equal(t, "default", ch.Group)
	assert.Equal(t, "active", ch.Level)
}

func TestGetChannel_EmptyChannelList(t *testing.T) {
	mgr := NewManager(nil)

	ch := mgr.GetChannel("anything")
	assert.Equal(t, "default", ch.Name)
	assert.Equal(t, "", ch.Sound)
	assert.Equal(t, "default", ch.Group)
	assert.Equal(t, "active", ch.Level)
}

func TestGetChannel_DefaultChannelDirectLookup(t *testing.T) {
	channels := []config.Channel{
		{Name: "default", Sound: "custom.caf", Group: "custom-group", Level: "timeSensitive"},
	}

	mgr := NewManager(channels)

	ch := mgr.GetChannel("default")
	assert.Equal(t, "default", ch.Name)
	assert.Equal(t, "custom.caf", ch.Sound)
	assert.Equal(t, "custom-group", ch.Group)
	assert.Equal(t, "timeSensitive", ch.Level)
}

func TestHasChannel(t *testing.T) {
	channels := []config.Channel{
		{Name: "urgent", Sound: "alarm.caf", Group: "紧急", Level: "critical"},
	}

	mgr := NewManager(channels)

	assert.True(t, mgr.HasChannel("urgent"))
	assert.False(t, mgr.HasChannel("nonexistent"))
}

func TestAll(t *testing.T) {
	channels := []config.Channel{
		{Name: "urgent", Sound: "alarm.caf", Group: "紧急", Level: "critical"},
		{Name: "github", Sound: "glass.caf", Group: "开发", Level: "active"},
	}

	mgr := NewManager(channels)

	all := mgr.All()
	assert.Len(t, all, 2)

	names := make(map[string]bool)
	for _, ch := range all {
		names[ch.Name] = true
	}
	assert.True(t, names["urgent"])
	assert.True(t, names["github"])
}
