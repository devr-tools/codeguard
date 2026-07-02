package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// shiftPromptBody is a prompt fixture whose secret-interpolation finding sits
// two lines deep, so the finding's full ±2 context window lives inside the
// fixture and inserting lines above the window shifts the finding without
// changing its surrounding source.
const shiftPromptBody = "context line one\n" +
	"context line two\n" +
	"Use ${OPENAI_API_KEY} for downstream calls.\n" +
	"context line four\n" +
	"context line five\n"

func promptOnlyConfig(dir string, name string) codeguard.Config {
	cfg := codeguard.ExampleConfig()
	cfg.Name = name
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Prompts = true
	cfg.Checks.Design = false
	cfg.Checks.Quality = false
	cfg.Checks.Security = false
	cfg.Checks.CI = false
	return cfg
}

func findFindingByRule(t *testing.T, report codeguard.Report, ruleID string) codeguard.Finding {
	t.Helper()
	for _, section := range report.Sections {
		for _, finding := range section.Findings {
			if finding.RuleID == ruleID {
				return finding
			}
		}
	}
	t.Fatalf("finding for rule %q not found", ruleID)
	return codeguard.Finding{}
}

// A baseline recorded before an unrelated edit must keep suppressing a finding
// whose line number shifted: the legacy fingerprint changes with the line, but
// the context fingerprint (rule, path, normalized surrounding source) does not.
func TestBaselineSuppressesFindingAfterLineShift(t *testing.T) {
	dir := t.TempDir()
	promptPath := filepath.Join(dir, "prompts", "system.prompt")
	writeFile(t, promptPath, shiftPromptBody)

	cfg := promptOnlyConfig(dir, "fingerprint-shift-test")

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertSectionStatus(t, report, "AI Prompts", "fail")
	before := findFindingByRule(t, report, "prompts.secret-interpolation")
	if before.ContextFingerprint == "" {
		t.Fatal("expected finding to carry a context fingerprint")
	}
	if before.ContextFingerprint == before.Fingerprint {
		t.Fatal("expected context fingerprint to differ from the legacy line-based fingerprint")
	}

	baselinePath := filepath.Join(dir, "codeguard-baseline.json")
	if writeErr := codeguard.WriteBaselineFile(baselinePath, codeguard.BaselineEntriesFromReport(report)); writeErr != nil {
		t.Fatalf("write baseline: %v", writeErr)
	}

	// Unrelated edit: insert lines above the finding's context window.
	writeFile(t, promptPath, "inserted header line\nanother inserted line\n"+shiftPromptBody)

	report, err = codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run after edit: %v", err)
	}
	after := findFindingByRule(t, report, "prompts.secret-interpolation")
	if after.Fingerprint == before.Fingerprint {
		t.Error("expected legacy fingerprint to change when the finding line shifts")
	}
	if after.ContextFingerprint != before.ContextFingerprint {
		t.Errorf("context fingerprint changed across a pure line shift: %q -> %q", before.ContextFingerprint, after.ContextFingerprint)
	}

	cfg.Baseline.Path = baselinePath
	report, err = codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run with baseline: %v", err)
	}
	assertSectionStatus(t, report, "AI Prompts", "pass")
	if report.Summary.SuppressedFindings == 0 {
		t.Fatal("expected the pre-edit baseline to suppress the shifted finding")
	}
}

// Baseline files written before context fingerprints existed carry legacy-only
// entries; they must keep suppressing unchanged findings.
func TestLegacyOnlyBaselineStillSuppresses(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "prompts", "system.prompt"), shiftPromptBody)

	cfg := promptOnlyConfig(dir, "fingerprint-legacy-test")

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertSectionStatus(t, report, "AI Prompts", "fail")

	entries := codeguard.BaselineEntriesFromReport(report)
	for i := range entries {
		entries[i].ContextFingerprint = ""
	}
	baselinePath := filepath.Join(dir, "codeguard-baseline.json")
	if writeErr := codeguard.WriteBaselineFile(baselinePath, entries); writeErr != nil {
		t.Fatalf("write baseline: %v", writeErr)
	}

	cfg.Baseline.Path = baselinePath
	report, err = codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run with baseline: %v", err)
	}
	assertSectionStatus(t, report, "AI Prompts", "pass")
	if report.Summary.SuppressedFindings == 0 {
		t.Fatal("expected legacy-only baseline entries to keep suppressing the finding")
	}
}
