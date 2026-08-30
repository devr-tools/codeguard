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

	codeguardcli "github.com/devr-tools/codeguard/internal/cli"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
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

func findPromptSecretFinding(t *testing.T, report codeguard.Report) codeguard.Finding {
	t.Helper()
	for _, section := range report.Sections {
		for _, finding := range section.Findings {
			if finding.RuleID == "prompts.secret-interpolation" {
				return finding
			}
		}
	}
	t.Fatal("prompts.secret-interpolation finding not found")
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
	before := findPromptSecretFinding(t, report)
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
	after := findPromptSecretFinding(t, report)
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

	current := findPromptSecretFinding(t, report)
	preV2Sum := sha256.Sum256([]byte(strings.Join([]string{
		current.RuleID,
		filepath.ToSlash(current.Path),
		strconv.Itoa(current.Line),
		current.Message,
	}, "|")))
	entries := []codeguard.BaselineEntry{{
		Fingerprint: hex.EncodeToString(preV2Sum[:]),
		RuleID:      current.RuleID,
		Path:        current.Path,
		Message:     current.Message,
	}}
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
	current := findPromptSecretFinding(t, report)
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

func TestBaselineAuditAndPruneKeepWordingIndependentMatchesAndExposeResolvedFalsePositives(t *testing.T) {
	dir := t.TempDir()
	const source = "package sample\n\nfunc Current(input *State) int {\n\tinput.Value++\n\treturn input.Value\n}\n"
	writeFile(t, filepath.Join(dir, "state.go"), source)
	sc := runnersupport.Context{Cfg: core.Config{Targets: []core.TargetConfig{{Name: "repo", Path: dir}}}}
	prior := runnersupport.NewFinding(sc, runnersupport.FindingInput{
		RuleID: "function.hidden-mutation", Path: "state.go", Line: 4,
		Message: "function Current mutates argument state", Confidence: "medium",
		Metadata: map[string]string{"mutation_target": "argument", "origin": "caller_owned"},
	})
	current := runnersupport.NewFinding(sc, runnersupport.FindingInput{
		RuleID: "function.hidden-mutation", Path: "state.go", Line: 4,
		Message: "Current changes caller-owned state", Confidence: "high",
		Metadata: map[string]string{"mutation_target": "argument", "effect_kind": "shared_state", "origin": "caller_owned"},
	})
	if prior.Fingerprint != current.Fingerprint || prior.ContextFingerprint != current.ContextFingerprint || prior.ContentFingerprint != current.ContentFingerprint {
		t.Fatalf("wording/evidence changed source identity:\nprior=%#v\ncurrent=%#v", prior, current)
	}
	priorExact := sha256.Sum256([]byte(strings.Join([]string{
		prior.RuleID,
		filepath.ToSlash(prior.Path),
		strconv.Itoa(prior.Line),
		prior.Message,
	}, "|")))
	active := core.BaselineEntry{
		Fingerprint:        hex.EncodeToString(priorExact[:]),
		ContextFingerprint: prior.ContextFingerprint,
		ContentFingerprint: prior.ContentFingerprint,
		RuleID:             prior.RuleID,
		Path:               prior.Path,
		Message:            prior.Message,
	}
	resolvedFalsePositive := core.BaselineEntry{
		Fingerprint: "resolved-structural-false-positive",
		RuleID:      "function.hidden-mutation",
		Path:        "helpers.go",
		Message:     "local value construction was misclassified as shared mutation",
	}
	audit := codeguardcli.Audit(core.BaselineFile{Entries: []core.BaselineEntry{active, resolvedFalsePositive}}, []core.Finding{current}, codeguardcli.Options{})
	if audit.Counts.ActiveContext != 1 || audit.Counts.Active != 1 || audit.Counts.Stale != 1 || audit.Counts.Final != 1 {
		t.Fatalf("audit counts = %#v, want one context-active and one stale entry", audit.Counts)
	}
	retained := audit.ActiveEntries()
	if len(retained) != 1 || retained[0].Fingerprint != active.Fingerprint {
		t.Fatalf("prune retained = %#v, want only wording-independent active entry", retained)
	}
	prunable := audit.PrunableEntries()
	if len(prunable) != 1 || prunable[0].Fingerprint != resolvedFalsePositive.Fingerprint {
		t.Fatalf("prunable = %#v, want only resolved structural false positive", prunable)
	}
}

func TestPreV2ExactOnlyBaselineSuppressesFindingWithoutSourceContext(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "short.go"), []byte("package sample\n"), 0o644); err != nil {
		t.Fatalf("write short source: %v", err)
	}
	cases := []struct {
		name string
		path string
		line int
	}{
		{name: "pathless", line: 0},
		{name: "unreadable", path: "missing.go", line: 3},
		{name: "invalid line", path: "short.go", line: 99},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			const message = "pre-v2 diagnostic prose"
			sum := sha256.Sum256([]byte(strings.Join([]string{"test.rule", filepath.ToSlash(tc.path), strconv.Itoa(tc.line), message}, "|")))
			preV2Exact := hex.EncodeToString(sum[:])
			entry := core.BaselineEntry{Fingerprint: preV2Exact, RuleID: "test.rule", Path: tc.path, Message: message}
			sc := runnersupport.Context{
				Cfg:      core.Config{Targets: []core.TargetConfig{{Name: "repo", Path: dir}}},
				Baseline: map[string]core.BaselineEntry{preV2Exact: entry},
			}
			finding := runnersupport.NewFinding(sc, runnersupport.FindingInput{
				RuleID: "test.rule", Path: tc.path, Line: tc.line, Message: message,
			})
			if finding.ContextFingerprint != finding.Fingerprint || finding.ContentFingerprint != "" {
				t.Fatalf("fixture unexpectedly has source fallback: %#v", finding)
			}
			suppression := runnersupport.MatchSuppression(sc, finding)
			if suppression == nil || suppression.Kind != runnersupport.SuppressionReasonBaseline || suppression.Match != "exact" || suppression.BaselineFingerprint != preV2Exact {
				t.Fatalf("suppression = %#v, want pre-v2 exact baseline match", suppression)
			}
		})
	}
}
