package support

import (
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/internal/codeguard/rules"
)

// DropDisabledRuleFindings removes findings of rules listed in
// checks.disabled_rules from sections that do not pass through
// FinalizeSection (imported external reports), recomputing each touched
// section's status from what remains. Operational diagnostics keep their
// effect on the status.
func DropDisabledRuleFindings(sections []core.SectionResult, patterns []string) []core.SectionResult {
	if len(patterns) == 0 {
		return sections
	}
	out := make([]core.SectionResult, len(sections))
	for idx, section := range sections {
		kept := make([]core.Finding, 0, len(section.Findings))
		for _, finding := range section.Findings {
			if _, disabled := rules.MatchDisabledRule(patterns, finding.RuleID); disabled {
				continue
			}
			kept = append(kept, finding)
		}
		if len(kept) == len(section.Findings) {
			out[idx] = section
			continue
		}
		section.Findings = kept
		section.Status = core.StatusPass
		for _, finding := range kept {
			section.Status = escalateSectionStatus(section.Status, finding.Level)
		}
		for _, diagnostic := range section.Diagnostics {
			if diagnostic.Operational {
				section.Status = escalateSectionStatus(section.Status, diagnostic.Level)
			}
		}
		out[idx] = section
	}
	return out
}
