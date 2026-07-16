package agentcontext

import (
	"fmt"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// legibilityThresholdFindings turns the repo_legibility score into an
// enforceable gate. Legibility is good-high (the inverse of the slop score),
// so the finding fires when the score falls BELOW a configured threshold:
// context_rules.legibility_fail_threshold fails the scan,
// legibility_warn_threshold warns, and 0 (the default) disables each
// threshold. The fail threshold takes precedence when both match.
func legibilityThresholdFindings(env support.Context, legibility *core.RepoLegibilityArtifact) []core.Finding {
	if legibility == nil {
		return nil
	}
	rules := env.Config.Checks.ContextRules
	var level string
	var threshold int
	switch {
	case rules.LegibilityFailThreshold > 0 && legibility.Score < rules.LegibilityFailThreshold:
		level, threshold = "fail", rules.LegibilityFailThreshold
	case rules.LegibilityWarnThreshold > 0 && legibility.Score < rules.LegibilityWarnThreshold:
		level, threshold = "warn", rules.LegibilityWarnThreshold
	default:
		return nil
	}
	return []core.Finding{env.NewFinding(support.FindingInput{
		RuleID: "context.legibility-threshold",
		Level:  level,
		Message: fmt.Sprintf(
			"repository legibility score %d fell below the configured %s threshold %d (%s); "+
				"improve the weakest components or adjust context_rules.legibility_%s_threshold",
			legibility.Score, level, threshold,
			formatLegibilityComponents(legibility.Components), level),
	})}
}

// formatLegibilityComponents renders the component breakdown for the
// threshold finding message, e.g. "agent_docs 10/25, readme 10/10, ...".
func formatLegibilityComponents(components []core.RepoLegibilityComponent) string {
	parts := make([]string, 0, len(components))
	for _, component := range components {
		parts = append(parts, fmt.Sprintf("%s %d/%d", component.Label, component.Score, component.Max))
	}
	return strings.Join(parts, ", ")
}
