package support_test

import (
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

func demotionContext(t testing.TB, demote bool) runnersupport.Context {
	t.Helper()
	sc := confidenceContext(t, core.ConfidencePolicyConfig{})
	sc.Cfg.Checks.ConfidenceDemotion = demote
	return sc
}

func TestConfidenceDemotionLowersFailingLowConfidenceFindings(t *testing.T) {
	sc := demotionContext(t, true)
	findings := []core.Finding{confidenceFinding(sc, confidenceRuleLow, core.ConfidenceLow)}
	section := runnersupport.FinalizeSection(sc, "security", "Security", findings)
	if len(section.Findings) != 1 {
		t.Fatalf("findings = %d, want 1: demotion must not remove findings", len(section.Findings))
	}
	got := section.Findings[0]
	if got.Level != "warn" || got.Severity != "warn" {
		t.Fatalf("level/severity = %q/%q, want warn/warn", got.Level, got.Severity)
	}
	if section.Status != core.StatusWarn {
		t.Fatalf("status = %q, want %q", section.Status, core.StatusWarn)
	}
}

func TestConfidenceDemotionOffByDefault(t *testing.T) {
	sc := demotionContext(t, false)
	findings := []core.Finding{confidenceFinding(sc, confidenceRuleLow, core.ConfidenceLow)}
	section := runnersupport.FinalizeSection(sc, "security", "Security", findings)
	if section.Findings[0].Level != "fail" || section.Status != core.StatusFail {
		t.Fatalf("level = %q status = %q, want fail/fail", section.Findings[0].Level, section.Status)
	}
}

// Demotion applies only to low confidence, and only downward.
func TestConfidenceDemotionLeavesOtherFindingsAlone(t *testing.T) {
	sc := demotionContext(t, true)
	tests := []struct {
		name       string
		confidence string
		level      string
		want       string
	}{
		{name: "medium confidence keeps fail", confidence: core.ConfidenceMedium, level: "fail", want: "fail"},
		{name: "high confidence keeps fail", confidence: core.ConfidenceHigh, level: "fail", want: "fail"},
		{name: "unspecified confidence keeps fail", confidence: "", level: "fail", want: "fail"},
		{name: "low confidence warn is not promoted", confidence: core.ConfidenceLow, level: "warn", want: "warn"},
		{name: "low confidence pass is untouched", confidence: core.ConfidenceLow, level: "pass", want: "pass"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			finding := runnersupport.NewFinding(sc, runnersupport.FindingInput{
				RuleID:     confidenceRuleLow,
				Level:      tt.level,
				Path:       "app/main.go",
				Line:       10,
				Message:    "demo finding",
				Confidence: tt.confidence,
			})
			section := runnersupport.FinalizeSection(sc, "security", "Security", []core.Finding{finding})
			if got := section.Findings[0].Level; got != tt.want {
				t.Fatalf("level = %q, want %q", got, tt.want)
			}
		})
	}
}

// Demotion changes how loudly a finding is reported, never what it is, so an
// existing baseline entry must keep matching after the flag is switched on.
func TestConfidenceDemotionPreservesFindingIdentity(t *testing.T) {
	plain := demotionContext(t, false)
	demoting := demotionContext(t, true)
	input := runnersupport.FindingInput{
		RuleID:     confidenceRuleLow,
		Path:       "app/main.go",
		Line:       10,
		Message:    "demo finding",
		Confidence: core.ConfidenceLow,
	}
	undemoted := runnersupport.FinalizeSection(plain, "security", "Security",
		[]core.Finding{runnersupport.NewFinding(plain, input)}).Findings[0]
	demoted := runnersupport.FinalizeSection(demoting, "security", "Security",
		[]core.Finding{runnersupport.NewFinding(demoting, input)}).Findings[0]

	if demoted.Level == undemoted.Level {
		t.Fatalf("demotion did not change the reported level (both %q)", demoted.Level)
	}
	if demoted.Fingerprint != undemoted.Fingerprint ||
		demoted.ContextFingerprint != undemoted.ContextFingerprint ||
		demoted.ContentFingerprint != undemoted.ContentFingerprint {
		t.Fatal("confidence demotion changed finding identity")
	}
}

// Suppression is matched on identity, so a baselined finding stays suppressed
// whether or not it is demoted.
func TestConfidenceDemotionKeepsBaselineMatching(t *testing.T) {
	sc := demotionContext(t, true)
	finding := confidenceFinding(sc, confidenceRuleLow, core.ConfidenceLow)
	sc.Baseline = map[string]core.BaselineEntry{
		finding.Fingerprint: {Fingerprint: finding.Fingerprint},
	}
	section := runnersupport.FinalizeSection(sc, "security", "Security", []core.Finding{finding})
	if len(section.Findings) != 0 || section.SuppressedCount != 1 {
		t.Fatalf("findings = %d suppressed = %d, want 0/1", len(section.Findings), section.SuppressedCount)
	}
}
