package support_test

import (
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

// Several single-language targets over one repository root make the
// language-agnostic passes emit the same finding once per target. A section
// reports such a finding once, while distinct findings on the same line stay
// separate.
func TestFinalizeSectionCollapsesIdenticalFindings(t *testing.T) {
	sc := confidenceContext(t, core.ConfidencePolicyConfig{})
	same := confidenceFinding(sc, confidenceRuleHigh, core.ConfidenceHigh)
	other := runnersupport.NewFinding(sc, runnersupport.FindingInput{
		RuleID:     confidenceRuleHigh,
		Path:       "app/main.go",
		Line:       10,
		Column:     1,
		Message:    "a different problem on the same line",
		Confidence: core.ConfidenceHigh,
	})

	section := runnersupport.FinalizeSection(sc, "security", "Security", []core.Finding{same, same, other, same})
	if len(section.Findings) != 2 {
		t.Fatalf("findings = %d, want 2 (one per distinct finding): %+v", len(section.Findings), section.Findings)
	}
	if got := statsEntry(t, sc, confidenceRuleHigh).Emitted; got != 2 {
		t.Fatalf("emitted = %d, want 2: a collapsed duplicate must not be counted", got)
	}
}
