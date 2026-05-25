package router

import (
	"fmt"

	"github.com/overseer/overseer/internal/config"
	"github.com/overseer/overseer/internal/model"
)

// Router routes incoming messages to channels based on configured rules.
type Router struct {
	rules    []CompiledRule
	channels map[string]*config.Channel
	fallback *config.Channel
}

// defaultFallbackChannel is used when no "default" channel is configured.
var defaultFallbackChannel = &config.Channel{
	Name:  "default",
	Sound: "",
	Group: "default",
	Level: "active",
}

// NewRouter creates a Router from the given rules and channels.
// Rules are compiled in order; an error is returned if any rule's content regex is invalid.
func NewRouter(rules []config.Rule, channels []config.Channel) (*Router, error) {
	compiled := make([]CompiledRule, 0, len(rules))
	for _, r := range rules {
		cr, err := compileRule(r)
		if err != nil {
			return nil, fmt.Errorf("rule %q: invalid content regex %q: %w", r.Name, r.Content, err)
		}
		compiled = append(compiled, *cr)
	}

	channelMap := make(map[string]*config.Channel, len(channels))
	for i := range channels {
		channelMap[channels[i].Name] = &channels[i]
	}

	fallback := defaultFallbackChannel
	if ch, ok := channelMap["default"]; ok {
		fallback = ch
	}

	return &Router{
		rules:    compiled,
		channels: channelMap,
		fallback: fallback,
	}, nil
}

// Route matches the message against rules in order (first-match wins)
// and returns the target channel and template name.
// If no rule matches, the message is routed to the "default" channel.
func (r *Router) Route(msg *model.Message) (*config.Channel, string) {
	for i := range r.rules {
		rule := &r.rules[i]
		if rule.matches(msg.Source, msg.Body) {
			ch := r.resolveChannel(rule.TargetChannel)
			return ch, rule.TemplateName
		}
	}
	// No rule matched — fall back to default channel.
	return r.fallback, ""
}

// resolveChannel looks up a channel by name.
// If the channel doesn't exist, it returns the fallback channel.
func (r *Router) resolveChannel(name string) *config.Channel {
	if ch, ok := r.channels[name]; ok {
		return ch
	}
	return r.fallback
}
