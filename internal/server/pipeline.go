package server

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/overseer/overseer/internal/aggregator"
	"github.com/overseer/overseer/internal/config"
	"github.com/overseer/overseer/internal/escalator"
	"github.com/overseer/overseer/internal/model"
	"github.com/overseer/overseer/internal/pusher"
	"github.com/overseer/overseer/internal/router"
	"github.com/overseer/overseer/internal/scheduler"
	"github.com/overseer/overseer/internal/store"
	"github.com/overseer/overseer/internal/template"
	"github.com/overseer/overseer/internal/ws"
)

// Pipeline wires all message processing components together:
// Webhook/Push → Router → Template → Aggregator → Scheduler → Pusher → DB → WebSocket Hub.
type Pipeline struct {
	router       *router.Router
	templateEng  *template.Engine
	aggregator   *aggregator.Aggregator
	scheduler    *scheduler.DNDScheduler
	pusher       *pusher.BarkPusher
	store        store.Store
	hub          *ws.Hub
	escalator    *escalator.Escalator
	cfg          *config.Config
}

// PipelineConfig holds all dependencies needed to construct a Pipeline.
type PipelineConfig struct {
	Router      *router.Router
	Template    *template.Engine
	Aggregator  *aggregator.Aggregator
	Scheduler   *scheduler.DNDScheduler
	Pusher      *pusher.BarkPusher
	Store       store.Store
	Hub         *ws.Hub
	Escalator   *escalator.Escalator
	Config      *config.Config
}

// NewPipeline creates a Pipeline from the given dependencies.
func NewPipeline(pc PipelineConfig) *Pipeline {
	return &Pipeline{
		router:      pc.Router,
		templateEng: pc.Template,
		aggregator:  pc.Aggregator,
		scheduler:   pc.Scheduler,
		pusher:      pc.Pusher,
		store:       pc.Store,
		hub:         pc.Hub,
		escalator:   pc.Escalator,
		cfg:         pc.Config,
	}
}

// ProcessMessage routes a message through the full pipeline:
// 1. Route → 2. Template render → 3. Aggregator → 4. DND Scheduler → 5. Push → 6. DB save → 7. WS broadcast → 8. Escalator track.
func (p *Pipeline) ProcessMessage(msg *model.Message) error {
	// 1. Route the message to determine target channel and template name.
	channel, templateName := p.router.Route(msg)
	msg.Channel = channel.Name

	// 2. If a template is specified, render the message body.
	if templateName != "" && p.templateEng != nil {
		rendered, err := p.templateEng.Render(templateName, msg)
		if err != nil {
			log.Printf("[WARN] pipeline: template render error for %q: %v", templateName, err)
			// Fallback: keep original body (Render already handles this internally)
		} else {
			msg.Body = rendered
		}
	}

	// 3. Pass through the Aggregator.
	aggResult := p.aggregator.Process(msg, channel.Level)
	switch aggResult.Action {
	case aggregator.ActionDedupe:
		// Message is a duplicate — suppress it, no push needed.
		log.Printf("[INFO] pipeline: message %s deduplicated (count=%d)", msg.ID, aggResult.DupeCount)
		return nil
	case aggregator.ActionBatch:
		// Messages were batched into a summary — push the summary.
		return p.pushBatchSummary(aggResult, channel)
	case aggregator.ActionRateLimit:
		// Message was rate-limited — it's queued internally by the aggregator.
		log.Printf("[INFO] pipeline: message %s rate-limited on channel %s", msg.ID, channel.Name)
		return nil
	case aggregator.ActionPass:
		// Continue with normal processing.
	}

	// 4. Check DND Scheduler — if should defer, enqueue and return.
	if p.scheduler.ShouldDefer(channel.Level, time.Now()) {
		p.scheduler.Enqueue(msg)
		log.Printf("[INFO] pipeline: message %s deferred (DND active)", msg.ID)
		return nil
	}

	// 5. Push via BarkPusher (using PushToChannel for multi-device support).
	pushErr := p.pushMessage(msg, channel)

	// 6. Save the message to the database with the push result.
	if saveErr := p.saveMessage(msg); saveErr != nil {
		log.Printf("[ERROR] pipeline: failed to save message %s: %v", msg.ID, saveErr)
		// Don't return here — still broadcast and track escalation.
	}

	// 7. Broadcast a push event via WebSocket Hub.
	p.broadcastPushEvent(msg)

	// 8. If escalation is enabled and the message requires confirmation, track it.
	if p.escalator != nil && p.cfg.Escalation.Enabled && channel.Level == "critical" {
		p.escalator.Track(msg)
	}

	return pushErr
}

// TestPushHandler processes a message through the pipeline but skips database recording.
// Used for POST /api/push/test.
func (p *Pipeline) TestPushHandler(msg *model.Message) error {
	// 1. Route the message.
	channel, templateName := p.router.Route(msg)
	msg.Channel = channel.Name

	// 2. Template render.
	if templateName != "" && p.templateEng != nil {
		rendered, err := p.templateEng.Render(templateName, msg)
		if err != nil {
			log.Printf("[WARN] pipeline: test push template render error: %v", err)
		} else {
			msg.Body = rendered
		}
	}

	// 3-4. Skip aggregation and DND for test pushes — push directly.

	// 5. Push via BarkPusher.
	pushReq := pusher.PushRequest{
		Title: msg.Title,
		Body:  msg.Body,
		Sound: msg.Sound,
		Icon:  msg.Icon,
		Group: msg.Group,
		Level: msg.Level,
		URL:   msg.URL,
	}

	results := p.pusher.PushToChannel(context.Background(), pushReq, configChannelToChannel(channel), p.cfg.Bark.DeviceKey)

	// Check if at least one device succeeded.
	var lastErr string
	anySuccess := false
	for _, r := range results {
		if r.Success {
			anySuccess = true
		} else if r.Error != "" {
			lastErr = r.Error
		}
	}

	if !anySuccess {
		return fmt.Errorf("test push failed: %s", lastErr)
	}

	return nil
}

// pushMessage sends the message to all devices on the channel and updates the message status.
func (p *Pipeline) pushMessage(msg *model.Message, channel *config.Channel) error {
	pushReq := pusher.PushRequest{
		Title: msg.Title,
		Body:  msg.Body,
		Sound: msg.Sound,
		Icon:  msg.Icon,
		Group: msg.Group,
		Level: msg.Level,
		URL:   msg.URL,
	}

	results := p.pusher.PushToChannel(context.Background(), pushReq, *channel, p.cfg.Bark.DeviceKey)

	// Determine overall result.
	var lastErr string
	anySuccess := false
	totalRetries := 0
	for _, r := range results {
		if r.Success {
			anySuccess = true
		} else if r.Error != "" {
			lastErr = r.Error
		}
	}

	now := time.Now()
	if anySuccess {
		msg.Status = model.StatusSuccess
		msg.PushedAt = &now
		msg.RetryCount = totalRetries
		return nil
	}

	msg.Status = model.StatusFailed
	msg.FailReason = lastErr
	msg.RetryCount = totalRetries
	return fmt.Errorf("push failed: %s", lastErr)
}

// saveMessage persists the message to the database.
func (p *Pipeline) saveMessage(msg *model.Message) error {
	if p.store == nil {
		return nil
	}

	if err := p.store.SaveMessage(msg); err != nil {
		return fmt.Errorf("save message: %w", err)
	}

	return nil
}

// broadcastPushEvent sends a push event to all connected WebSocket clients.
func (p *Pipeline) broadcastPushEvent(msg *model.Message) {
	if p.hub == nil {
		return
	}

	p.hub.Broadcast(ws.Event{
		Type: "push",
		Payload: map[string]interface{}{
			"id":      msg.ID,
			"source":  msg.Source,
			"channel": msg.Channel,
			"title":   msg.Title,
			"body":    msg.Body,
			"status":  string(msg.Status),
			"time":    msg.ReceivedAt.Format(time.RFC3339),
		},
	})
}

// pushBatchSummary pushes a batched summary message.
func (p *Pipeline) pushBatchSummary(result aggregator.AggregateResult, channel *config.Channel) error {
	if len(result.Messages) == 0 {
		return nil
	}

	// Create a summary message from the batch.
	summaryMsg := &model.Message{
		ID:         result.Messages[0].ID,
		Source:     result.Messages[0].Source,
		Channel:    channel.Name,
		Title:      fmt.Sprintf("[批量合并 %d条] %s", len(result.Messages), result.Messages[0].Title),
		Body:       result.Summary,
		Status:     model.StatusPending,
		ReceivedAt: time.Now(),
	}

	// Check DND before pushing the batch summary.
	if p.scheduler.ShouldDefer(channel.Level, time.Now()) {
		p.scheduler.Enqueue(summaryMsg)
		return nil
	}

	pushErr := p.pushMessage(summaryMsg, channel)

	if saveErr := p.saveMessage(summaryMsg); saveErr != nil {
		log.Printf("[ERROR] pipeline: failed to save batch summary: %v", saveErr)
	}

	p.broadcastPushEvent(summaryMsg)

	return pushErr
}

// configChannelToChannel converts a *config.Channel to config.Channel value.
func configChannelToChannel(ch *config.Channel) config.Channel {
	if ch == nil {
		return config.Channel{Name: "default", Level: "active"}
	}
	return *ch
}
