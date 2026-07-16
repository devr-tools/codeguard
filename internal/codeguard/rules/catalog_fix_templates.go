package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

// fixTemplates holds concrete, agent-actionable fix instructions per rule:
// a short imperative description plus a before/after snippet where one makes
// sense, classified as deterministic (mechanically applicable) or guided
// (requires judgment). They surface through explain --format=agent and the
// MCP explain tool. The entries live in catalog_fix_templates_*.go, split by
// rule family.
// Short aliases keep the per-family template maps readable.
const (
	deterministic = core.FixTemplateKindDeterministic
	guided        = core.FixTemplateKindGuided
)

var fixTemplates = mergeFixTemplates(
	qualityFixTemplates,
	qualityAIFixTemplates,
	performanceFixTemplates,
	performanceRegressionFixTemplates,
	securityFixTemplates,
	securityLanguageFixTemplates,
	designFixTemplates,
	miscFixTemplates,
	contextFixTemplates,
)

func mergeFixTemplates(parts ...map[string]core.FixTemplate) map[string]core.FixTemplate {
	total := 0
	for _, part := range parts {
		total += len(part)
	}
	merged := make(map[string]core.FixTemplate, total)
	for _, part := range parts {
		for id, template := range part {
			merged[id] = template
		}
	}
	return merged
}

func applyFixTemplate(meta core.RuleMetadata) core.RuleMetadata {
	if !meta.FixTemplate.IsZero() {
		return meta
	}
	if template, ok := fixTemplates[meta.ID]; ok {
		meta.FixTemplate = template
	}
	return meta
}
