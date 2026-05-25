package template

import (
	"bytes"
	"fmt"
	"log"
	"text/template"

	"github.com/overseer/overseer/internal/config"
	"github.com/overseer/overseer/internal/model"
)

// Engine manages message templates and renders push content.
type Engine struct {
	templates map[string]*template.Template
}

// NewEngine parses all configured templates and returns an Engine.
// Returns an error if any template has invalid Go template syntax.
func NewEngine(templates []config.Template) (*Engine, error) {
	e := &Engine{
		templates: make(map[string]*template.Template, len(templates)),
	}

	for _, t := range templates {
		tmpl, err := template.New(t.Name).Parse(t.Content)
		if err != nil {
			return nil, fmt.Errorf("template %q: %w", t.Name, err)
		}
		e.templates[t.Name] = tmpl
	}

	return e, nil
}

// Render renders a message using the named template. It provides the template
// with source, title, body, timestamp, and extra fields.
//
// Fallback behavior:
//   - If the template does not exist, returns msg.Body and logs a warning.
//   - If template execution fails, returns msg.Body and logs a warning.
//   - If the rendered result is empty, returns msg.Body and logs a warning.
func (e *Engine) Render(name string, msg *model.Message) (string, error) {
	tmpl, ok := e.templates[name]
	if !ok {
		log.Printf("[WARN] template %q not found, using original message body", name)
		return msg.Body, nil
	}

	data := map[string]interface{}{
		"source":    msg.Source,
		"title":     msg.Title,
		"body":      msg.Body,
		"timestamp": msg.ReceivedAt.Format("2006-01-02 15:04:05"),
		"extra":     msg.Extra,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		log.Printf("[WARN] template %q execution error: %v, using original message body", name, err)
		return msg.Body, nil
	}

	result := buf.String()
	if result == "" {
		log.Printf("[WARN] template %q rendered empty result, using original message body", name)
		return msg.Body, nil
	}

	return result, nil
}
