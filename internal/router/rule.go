package router

import (
	"regexp"

	"github.com/overseer/overseer/internal/config"
)

// CompiledRule is a pre-compiled routing rule ready for matching.
type CompiledRule struct {
	Name          string
	SourceMatch   string         // exact match on message source
	ContentRegex  *regexp.Regexp // regex match on message body
	TargetChannel string
	TemplateName  string
}

// compileRule compiles a config.Rule into a CompiledRule.
// Returns an error if the content regex is invalid.
func compileRule(r config.Rule) (*CompiledRule, error) {
	cr := &CompiledRule{
		Name:          r.Name,
		SourceMatch:   r.Source,
		TargetChannel: r.Channel,
		TemplateName:  r.Template,
	}

	if r.Content != "" {
		re, err := regexp.Compile(r.Content)
		if err != nil {
			return nil, err
		}
		cr.ContentRegex = re
	}

	return cr, nil
}

// matches checks whether the given source and body match this rule.
// - If SourceMatch is set, source must match exactly.
// - If ContentRegex is set, body must match the regex.
// - When both are set, both must match (AND logic).
// - If neither is set, the rule matches any message.
func (cr *CompiledRule) matches(source, body string) bool {
	if cr.SourceMatch != "" && cr.SourceMatch != source {
		return false
	}
	if cr.ContentRegex != nil && !cr.ContentRegex.MatchString(body) {
		return false
	}
	return true
}
