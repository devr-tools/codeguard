package checks_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	report, err = codeguard.RunWithOptions(context.Background(), cfg, codeguard.ScanOptions{Mode: codeguard.ScanModeFull, IncludeSuppressed: true})
	if err != nil {
		t.Fatalf("run with baseline: %v", err)
	}
	assertSectionStatus(t, report, "AI Prompts", "pass")
	if report.Summary.SuppressedFindings == 0 {
		t.Fatal("expected the pre-edit baseline to suppress the shifted finding")
	}
	assertSuppressionMatch(t, report, "context")
}

func TestBaselineSuppressesFindingAfterMoveToSplitFile(t *testing.T) {
	dir := t.TempDir()
	legacyPath := filepath.Join(dir, "prompts", "legacy", "system.prompt")
	writeFile(t, legacyPath, shiftPromptBody)

	cfg := promptOnlyConfig(dir, "fingerprint-move-test")

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertSectionStatus(t, report, "AI Prompts", "fail")

	baselinePath := filepath.Join(dir, "codeguard-baseline.json")
	if writeErr := codeguard.WriteBaselineFile(baselinePath, codeguard.BaselineEntriesFromReport(report)); writeErr != nil {
		t.Fatalf("write baseline: %v", writeErr)
	}

	if removeErr := os.Remove(legacyPath); removeErr != nil {
		t.Fatalf("remove legacy file: %v", removeErr)
	}
	writeFile(t, filepath.Join(dir, "prompts", "split", "system.prompt"), shiftPromptBody)

	cfg.Baseline.Path = baselinePath
	report, err = codeguard.RunWithOptions(context.Background(), cfg, codeguard.ScanOptions{Mode: codeguard.ScanModeFull, IncludeSuppressed: true})
	if err != nil {
		t.Fatalf("run with baseline after move: %v", err)
	}
	assertSectionStatus(t, report, "AI Prompts", "pass")
	if report.Summary.SuppressedFindings == 0 {
		t.Fatal("expected the pre-move baseline to suppress the moved finding")
	}
	assertSuppressionMatch(t, report, "content")
}

func assertSuppressionMatch(t *testing.T, report codeguard.Report, want string) {
	t.Helper()
	for _, finding := range report.SuppressedFindings {
		if finding.RuleID == "prompts.secret-interpolation" && finding.Suppression != nil && finding.Suppression.Match == want {
			return
		}
	}
	t.Fatalf("suppressed findings did not contain %q match: %#v", want, report.SuppressedFindings)
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

// A production change that removes context/content fallback from baseline
// matching must fail this test. The exact value deliberately uses the prior
// rule|path|line|message identity while the current finding carries changed
// prose and evidence.
func TestBaselineFallsBackFromPriorExactFingerprintAfterEvidenceChange(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "prompts", "system.prompt"), shiftPromptBody)
	cfg := promptOnlyConfig(dir, "fingerprint-evidence-test")

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	current := findFindingByRule(t, report, "prompts.secret-interpolation")
	priorMessage := "prior finding prose with different evidence"
	priorSum := sha256.Sum256([]byte(strings.Join([]string{
		current.RuleID,
		filepath.ToSlash(current.Path),
		strconv.Itoa(current.Line),
		priorMessage,
	}, "|")))
	entry := codeguard.BaselineEntry{
		Fingerprint:        hex.EncodeToString(priorSum[:]),
		ContextFingerprint: current.ContextFingerprint,
		ContentFingerprint: current.ContentFingerprint,
		RuleID:             current.RuleID,
		Path:               current.Path,
		Message:            priorMessage,
	}
	if entry.Fingerprint == current.Fingerprint {
		t.Fatal("fixture prior exact fingerprint unexpectedly matches current exact fingerprint")
	}
	baselinePath := filepath.Join(dir, "codeguard-baseline.json")
	if writeErr := codeguard.WriteBaselineFile(baselinePath, []codeguard.BaselineEntry{entry}); writeErr != nil {
		t.Fatalf("write baseline: %v", writeErr)
	}

	cfg.Baseline.Path = baselinePath
	report, err = codeguard.RunWithOptions(context.Background(), cfg, codeguard.ScanOptions{Mode: codeguard.ScanModeFull, IncludeSuppressed: true})
	if err != nil {
		t.Fatalf("run with baseline: %v", err)
	}
	assertSectionStatus(t, report, "AI Prompts", "pass")
	assertSuppressionMatch(t, report, "context")
}
