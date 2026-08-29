package codeguard_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// TestRunPublishesRuleStatsArtifact scans a fixture where one custom rule
// fires four times: one finding is kept, one is baselined, one is waived, and
// one carries an inline codeguard:ignore. The rule_stats artifact must
// attribute each suppression to its mechanism and flow through JSON report
// serialization like every other artifact.
func TestRunPublishesRuleStatsArtifact(t *testing.T) {
	root := t.TempDir()
	writeArtifactFile(t, filepath.Join(root, "keep.go"), "package keep\n// TODO keep\n")
	writeArtifactFile(t, filepath.Join(root, "waived.go"), "package waived\n// TODO waived\n")
	writeArtifactFile(t, filepath.Join(root, "base.go"), "package base\n// TODO base\n")
	writeArtifactFile(t, filepath.Join(root, "inline.go"), "package inline\n// TODO inline codeguard:ignore custom.no-todo\n")

	baselinePath := filepath.Join(t.TempDir(), "codeguard-baseline.json")
	cfg := ruleStatsFixtureConfig(root, "")

	first, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("first Run returned error: %v", err)
	}
	writeRuleStatsBaseline(t, baselinePath, first, "base.go")

	report, err := codeguard.Run(context.Background(), ruleStatsFixtureConfig(root, baselinePath))
	if err != nil {
		t.Fatalf("second Run returned error: %v", err)
	}

	entry := findRuleStatsEntry(t, report, "custom.no-todo")
	want := codeguard.RuleStatsEntry{
		RuleID:             "custom.no-todo",
		Emitted:            1,
		BaselineSuppressed: 1,
		WaiverSuppressed:   1,
		InlineSuppressed:   1,
		SuppressionRatio:   0.75,
	}
	if entry != want {
		t.Fatalf("rule stats entry = %#v, want %#v", entry, want)
	}
	assertRuleStatsSerialized(t, report)
}

// Removing suppression collection or losing the mechanism/match metadata would
// make baseline audits unable to reconcile aggregate counts with real findings.
func TestRunCanIncludeStructuredSuppressedFindings(t *testing.T) {
	root := t.TempDir()
	writeArtifactFile(t, filepath.Join(root, "keep.go"), "package keep\n// TODO keep\n")
	writeArtifactFile(t, filepath.Join(root, "waived.go"), "package waived\n// TODO waived\n")
	writeArtifactFile(t, filepath.Join(root, "base.go"), "package base\n// TODO base\n")
	writeArtifactFile(t, filepath.Join(root, "inline.go"), "package inline\n// TODO inline codeguard:ignore custom.no-todo\n")

	cfg := ruleStatsFixtureConfig(root, "")
	first, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("first Run returned error: %v", err)
	}
	baselinePath := filepath.Join(t.TempDir(), "codeguard-baseline.json")
	writeRuleStatsBaseline(t, baselinePath, first, "base.go")
	cfg.Baseline.Path = baselinePath

	report, err := codeguard.RunWithOptions(context.Background(), cfg, codeguard.ScanOptions{
		Mode:              codeguard.ScanModeFull,
		IncludeSuppressed: true,
	})
	if err != nil {
		t.Fatalf("RunWithOptions returned error: %v", err)
	}
	if got, want := len(report.SuppressedFindings), report.Summary.SuppressedFindings; got != want {
		t.Fatalf("suppressed records = %d, summary = %d", got, want)
	}

	got := map[string]codeguard.Suppression{}
	for _, finding := range report.SuppressedFindings {
		got[finding.Path] = *finding.Suppression
	}
	if got["base.go"].Kind != "baseline" || got["base.go"].Match != "exact" || got["base.go"].BaselineFingerprint == "" {
		t.Fatalf("baseline suppression = %#v", got["base.go"])
	}
	if got["waived.go"].Kind != "waiver" {
		t.Fatalf("waiver suppression = %#v", got["waived.go"])
	}
	if got["inline.go"].Kind != "inline" {
		t.Fatalf("inline suppression = %#v", got["inline.go"])
	}
}

func ruleStatsFixtureConfig(root string, baselinePath string) codeguard.Config {
	cacheEnabled := false
	cfg := codeguard.Config{
		Name: "rule-stats-test",
		Targets: []codeguard.TargetConfig{{
			Name:     "repo",
			Path:     root,
			Language: "go",
		}},
		Output: codeguard.OutputConfig{Format: "json"},
		Cache:  codeguard.CacheConfig{Enabled: &cacheEnabled},
		RulePacks: []core.RulePackConfig{{
			Name: "repo-policy",
			Rules: []core.CustomRuleConfig{{
				ID:             "custom.no-todo",
				Title:          "No TODO comments",
				Severity:       "warn",
				Message:        "TODO comments must be tracked in the issue tracker",
				ContentRegex:   "TODO",
				FileExtensions: []string{".go"},
			}},
		}},
		Waivers: []codeguard.WaiverConfig{{
			Rule:   "custom.no-todo",
			Path:   "waived.go",
			Reason: "fixture waiver",
		}},
	}
	if baselinePath != "" {
		cfg.Baseline = codeguard.BaselineConfig{Path: baselinePath}
	}
	return cfg
}

// writeRuleStatsBaseline captures the fingerprints emitted for one path in a
// prior report and persists them as the baseline for the next scan.
func writeRuleStatsBaseline(t *testing.T, path string, report codeguard.Report, baselinedFile string) {
	t.Helper()
	entries := make([]codeguard.BaselineEntry, 0, 1)
	for _, entry := range codeguard.BaselineEntriesFromReport(report) {
		if entry.Path == baselinedFile {
			entries = append(entries, entry)
		}
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly one baseline entry for %s, got %#v", baselinedFile, entries)
	}
	if err := codeguard.WriteBaselineFile(path, entries); err != nil {
		t.Fatalf("WriteBaselineFile: %v", err)
	}
}

func findRuleStatsEntry(t *testing.T, report codeguard.Report, ruleID string) codeguard.RuleStatsEntry {
	t.Helper()
	for _, artifact := range report.Artifacts {
		if artifact.Kind != core.ReportArtifactKindRuleStats {
			continue
		}
		if artifact.ID != "rule_stats" {
			t.Fatalf("unexpected rule_stats artifact ID %q", artifact.ID)
		}
		if artifact.RuleStats == nil {
			t.Fatal("expected rule_stats payload")
		}
		for _, entry := range artifact.RuleStats.Rules {
			if entry.RuleID == ruleID {
				return entry
			}
		}
		t.Fatalf("rule %q missing from rule_stats artifact %#v", ruleID, artifact.RuleStats.Rules)
	}
	t.Fatalf("expected rule_stats artifact, got %#v", report.Artifacts)
	return codeguard.RuleStatsEntry{}
}

// assertRuleStatsSerialized proves the artifact survives report serialization
// the same way other artifacts do (no custom wiring in the JSON writer).
func assertRuleStatsSerialized(t *testing.T, report codeguard.Report) {
	t.Helper()
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	payload := string(data)
	for _, fragment := range []string{`"kind":"rule_stats"`, `"suppression_ratio":0.75`, `"baseline_suppressed":1`} {
		if !strings.Contains(payload, fragment) {
			t.Fatalf("expected serialized report to contain %s, got: %s", fragment, payload)
		}
	}
}
