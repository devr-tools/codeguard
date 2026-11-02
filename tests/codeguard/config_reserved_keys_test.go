package codeguard_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// runTriageFixture scans the triage fixture with a provider that dismisses
// every candidate, after mutate adjusts the config.
func runTriageFixture(t *testing.T, mutate func(*codeguard.Config)) (codeguard.Report, *codeguard.Artifact) {
	t.Helper()
	useTestTriageProvider(t, "fixture dismissal")
	cfg := triageFixtureConfig(t, t.TempDir())
	mutate(&cfg)
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	artifact := findAIAnalysisArtifact(report)
	if artifact == nil || artifact.AIAnalysis == nil {
		t.Fatalf("expected ai_analysis artifact, got %#v", report.Artifacts)
	}
	return report, artifact
}

func TestHybridTriageDefaultsDismissQualityFindings(t *testing.T) {
	report, artifact := runTriageFixture(t, func(*codeguard.Config) {})
	if findings := findSection(t, report, "Code Quality").Findings; len(findings) != 0 {
		t.Fatalf("default triage should drop dismissed findings, got %+v", findings)
	}
	if len(artifact.AIAnalysis.Verdicts) == 0 {
		t.Fatal("expected dismiss verdicts")
	}
}

func TestHybridTriageSuppressDismissedFalseKeepsFindings(t *testing.T) {
	off := false
	report, artifact := runTriageFixture(t, func(cfg *codeguard.Config) {
		cfg.AI.HybridTriage.SuppressDismissed = &off
	})
	if findings := findSection(t, report, "Code Quality").Findings; len(findings) == 0 {
		t.Fatal("suppress_dismissed=false must keep dismissed findings in the report")
	}
	if len(artifact.AIAnalysis.Verdicts) == 0 {
		t.Fatal("expected verdicts to be recorded in advisory mode")
	}
	for _, verdict := range artifact.AIAnalysis.Verdicts {
		if !strings.HasSuffix(verdict.Status, "dismissed") {
			t.Fatalf("verdict status = %q, want dismissed", verdict.Status)
		}
	}
}

func TestHybridTriageCandidateSectionsExcludesOtherSections(t *testing.T) {
	report, artifact := runTriageFixture(t, func(cfg *codeguard.Config) {
		cfg.AI.HybridTriage.CandidateSections = []string{"Security"}
	})
	if findings := findSection(t, report, "Code Quality").Findings; len(findings) == 0 {
		t.Fatal("findings outside candidate_sections must never be triaged away")
	}
	if len(artifact.AIAnalysis.Verdicts) != 0 {
		t.Fatalf("expected no verdicts for non-candidate sections, got %#v", artifact.AIAnalysis.Verdicts)
	}
}

func TestHybridTriageCandidateSectionsMatchesSectionID(t *testing.T) {
	report, _ := runTriageFixture(t, func(cfg *codeguard.Config) {
		cfg.AI.HybridTriage.CandidateSections = []string{"QUALITY"}
	})
	if findings := findSection(t, report, "Code Quality").Findings; len(findings) != 0 {
		t.Fatalf("section id should select the quality section, got %+v", findings)
	}
}

func TestHybridTriageCandidateSeveritiesExcludesOtherLevels(t *testing.T) {
	report, artifact := runTriageFixture(t, func(cfg *codeguard.Config) {
		cfg.AI.HybridTriage.CandidateSeverities = []string{"info"}
	})
	if findings := findSection(t, report, "Code Quality").Findings; len(findings) == 0 {
		t.Fatal("warn/fail findings must not be triaged when candidate_severities lists only info")
	}
	if len(artifact.AIAnalysis.Verdicts) != 0 {
		t.Fatalf("expected no verdicts, got %#v", artifact.AIAnalysis.Verdicts)
	}
}

func TestVerifyFixBatchHonorsMaxFixes(t *testing.T) {
	dir := t.TempDir()
	writeAPITestFile(t, filepath.Join(dir, "go.mod"), "module example.com/fixbatch\n\ngo 1.23.0\n")
	writeAPITestFile(t, filepath.Join(dir, "service.go"), "package fixbatch\n\nfunc run(){ }\n")
	writeAPITestFile(t, filepath.Join(dir, "other.go"), "package fixbatch\n\nfunc stop(){ }\n")
	writeAPITestFile(t, filepath.Join(dir, "service_test.go"), "package fixbatch\n\nimport \"testing\"\n\nfunc TestRun(t *testing.T) { run(); stop() }\n")
	gofmtDiff := func(file string, fn string) string {
		return strings.Join([]string{
			"diff --git a/" + file + " b/" + file,
			"--- a/" + file,
			"+++ b/" + file,
			"@@ -1,3 +1,3 @@",
			" package fixbatch",
			" ",
			"-func " + fn + "(){ }",
			"+func " + fn + "() {}",
			"",
		}, "\n")
	}
	cfg := qualityOnlyConfig(dir, "verify-fix-batch-max")
	cfg.AI.AutoFix.MaxFixes = 1
	result, err := codeguard.VerifyFixBatch(context.Background(), codeguard.FixBatchRequest{
		Config: cfg,
		Items: []codeguard.FixBatchItem{
			{Finding: codeguard.Finding{RuleID: "quality.gofmt", Fingerprint: "a"}, Candidate: codeguard.FixCandidate{Diff: gofmtDiff("service.go", "run")}},
			{Finding: codeguard.Finding{RuleID: "quality.gofmt", Fingerprint: "b"}, Candidate: codeguard.FixCandidate{Diff: gofmtDiff("other.go", "stop")}},
		},
	})
	if err != nil {
		t.Fatalf("verify fix batch: %v", err)
	}
	if got, want := result.Included, []int{0}; !slices.Equal(got, want) {
		t.Fatalf("included = %#v, want %#v", got, want)
	}
	if len(result.Skipped) != 1 || result.Skipped[0].Index != 1 || result.Skipped[0].Reason != codeguard.FixBatchReasonMaxFixes {
		t.Fatalf("skipped = %#v, want item 1 skipped as %s", result.Skipped, codeguard.FixBatchReasonMaxFixes)
	}
}

// deprecatedKeyConfigs sets each deprecated key in an otherwise valid YAML
// config; the value is the dotted key the deprecation must name.
var deprecatedKeyConfigs = map[string]string{
	"checks.reliability_rules.max_retry_attempts":           "checks:\n  reliability_rules:\n    max_retry_attempts: 3\n",
	"checks.data_rules.max_unbounded_read_rows":             "checks:\n  data_rules:\n    max_unbounded_read_rows: 1000\n",
	"checks.observability_rules.structured_logger_patterns": "checks:\n  observability_rules:\n    structured_logger_patterns: [\"logger.\"]\n",
	"checks.design_rules.forbid_internal_import_cmd":        "checks:\n  design_rules:\n    forbid_internal_import_cmd: true\n",
	"ai.autofix.verify_tests":                               "ai:\n  autofix:\n    verify_tests: true\n",
	"rule_packs[0].rules[0].ai_prompt":                      "rule_packs:\n  - name: pack\n    rules:\n      - id: custom.todo\n        title: TODO\n        message: todo found\n        content_regex: TODO\n        ai_prompt: is this a real TODO?\n",
}

func writeDeprecatedKeyConfig(t *testing.T, dir string, body string) string {
	t.Helper()
	path := filepath.Join(dir, "codeguard.yaml")
	base := "name: deprecated\ntargets:\n  - name: repo\n    path: .\n    language: go\n"
	if !strings.HasPrefix(body, "checks:") {
		base += "checks:\n  quality: true\n"
	}
	if err := os.WriteFile(path, []byte(base+body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestConfigDeprecationsNameEachDeprecatedKey(t *testing.T) {
	for key, body := range deprecatedKeyConfigs {
		t.Run(key, func(t *testing.T) {
			path := writeDeprecatedKeyConfig(t, t.TempDir(), body)
			if _, err := codeguard.LoadConfigFile(path); err != nil {
				t.Fatalf("deprecated key must keep loading: %v", err)
			}
			deprecations, err := codeguard.ConfigDeprecations(path)
			if err != nil {
				t.Fatalf("ConfigDeprecations: %v", err)
			}
			if len(deprecations) != 1 || deprecations[0].Key != key {
				t.Fatalf("deprecations = %#v, want one for %s", deprecations, key)
			}
			message := deprecations[0].Message()
			if !strings.Contains(message, key) || !strings.Contains(message, "removed in codeguard 2.0") {
				t.Fatalf("message %q should name the key and the removal release", message)
			}
		})
	}
}

func TestConfigDeprecationsIgnoresConfigsWithoutDeprecatedKeys(t *testing.T) {
	path := writeDeprecatedKeyConfig(t, t.TempDir(), "")
	deprecations, err := codeguard.ConfigDeprecations(path)
	if err != nil {
		t.Fatalf("ConfigDeprecations: %v", err)
	}
	if len(deprecations) != 0 {
		t.Fatalf("deprecations = %#v, want none", deprecations)
	}
}

func TestConfigDeprecationsInspectsExternalDesignRulesFile(t *testing.T) {
	dir := t.TempDir()
	path := writeDeprecatedKeyConfig(t, dir, "")
	if err := os.MkdirAll(filepath.Join(dir, ".codeguard"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".codeguard", "design_rules.yml"), []byte("forbid_internal_import_cmd: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	deprecations, err := codeguard.ConfigDeprecations(path)
	if err != nil {
		t.Fatalf("ConfigDeprecations: %v", err)
	}
	if len(deprecations) != 1 || deprecations[0].Key != "forbid_internal_import_cmd" || !strings.HasSuffix(deprecations[0].File, "design_rules.yml") {
		t.Fatalf("deprecations = %#v", deprecations)
	}
}

func TestWriteConfigFileOmitsDeprecatedDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "codeguard.yaml")
	if err := codeguard.WriteConfigFile(path, codeguard.ExampleConfig()); err != nil {
		t.Fatalf("WriteConfigFile: %v", err)
	}
	deprecations, err := codeguard.ConfigDeprecations(path)
	if err != nil {
		t.Fatalf("ConfigDeprecations: %v", err)
	}
	if len(deprecations) != 0 {
		t.Fatalf("written config carries deprecated keys: %#v", deprecations)
	}
}

// The list `init` wrote into every pre-1.12 config is treated as "every
// section": candidate_sections was never applied before, so honouring that
// list would silently stop triaging Reliability (and every other section not
// on it) for configs that never chose it.
func TestHybridTriageLegacyInitCandidateSectionsTriagesEverySection(t *testing.T) {
	reliability := func(sections []string) []codeguard.Finding {
		t.Helper()
		useTestTriageProvider(t, "fixture dismissal")
		root := t.TempDir()
		cfg := triageFixtureConfig(t, root)
		writeArtifactFile(t, filepath.Join(root, "errors.go"), "package sample\n\nimport \"io\"\n\nfunc Copy(dst io.Writer, src io.Reader) {\n\t_, _ = io.Copy(dst, src)\n\tpanic(\"copy failed\")\n}\n")
		on := true
		cfg.Checks.Reliability = &on
		cfg.AI.HybridTriage.CandidateSections = sections
		report, err := codeguard.Run(context.Background(), cfg)
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
		return findSection(t, report, "Reliability").Findings
	}

	if kept := reliability([]string{"Code Quality", "Design Patterns", "Security"}); len(kept) == 0 {
		t.Fatal("control: Reliability is outside a customised candidate list, so its findings must be kept")
	}
	legacy := []string{"security", "Custom Rules", "Code Quality", "Design Patterns"}
	if kept := reliability(legacy); len(kept) != 0 {
		t.Fatalf("the legacy init list must triage every section; Reliability findings survived: %+v", kept)
	}
	if kept := reliability(nil); len(kept) != 0 {
		t.Fatalf("an empty candidate list must triage every section; Reliability findings survived: %+v", kept)
	}
}
