package codeguard_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func coverageDeltaConfig(dir string, language string) codeguard.Config {
	enabled := true
	disabled := false
	cfg := codeguard.ExampleConfig()
	cfg.Name = "coverage-delta"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: language}}
	cfg.Checks.Design = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.QualityRules.CoverageDelta.Enabled = &enabled
	cfg.Cache.Enabled = &disabled
	return cfg
}

func runDiffScan(t *testing.T, cfg codeguard.Config) codeguard.Report {
	t.Helper()
	report, err := codeguard.RunWithOptions(context.Background(), cfg, codeguard.ScanOptions{
		Mode:    codeguard.ScanModeDiff,
		BaseRef: "main",
	})
	if err != nil {
		t.Fatalf("diff scan: %v", err)
	}
	return report
}

func coverageDeltaFindings(report codeguard.Report) []codeguard.Finding {
	findings := make([]codeguard.Finding, 0)
	for _, section := range report.Sections {
		for _, finding := range section.Findings {
			if finding.RuleID == "quality.coverage-delta" {
				findings = append(findings, finding)
			}
		}
	}
	return findings
}

func setupGoCoverageRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeRepoFile(t, filepath.Join(dir, "go.mod"), "module example.com/covdemo\n\ngo 1.23\n")
	writeRepoFile(t, filepath.Join(dir, "calc.go"), `package covdemo

func Add(a, b int) int {
	return a + b
}

func Sub(a, b int) int {
	return a - b
}
`)
	writeRepoFile(t, filepath.Join(dir, "calc_test.go"), `package covdemo

import "testing"

func TestAdd(t *testing.T) {
	if Add(1, 2) != 3 {
		t.Fatalf("Add(1, 2) = %d", Add(1, 2))
	}
}
`)
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "initial")
	runGit(t, dir, "checkout", "-b", "feature")
	return dir
}

func TestCoverageDeltaFlagsUncoveredChangedGoLines(t *testing.T) {
	if testing.Short() {
		t.Skip("runs go test in a fixture module")
	}
	dir := setupGoCoverageRepo(t)
	// Sub stays untested; the added Mul is untested too.
	writeRepoFile(t, filepath.Join(dir, "calc.go"), `package covdemo

func Add(a, b int) int {
	return a + b
}

func Sub(a, b int) int {
	return a - b - 0
}

func Mul(a, b int) int {
	return a * b
}
`)

	report := runDiffScan(t, coverageDeltaConfig(dir, "go"))

	findings := coverageDeltaFindings(report)
	if len(findings) != 1 {
		t.Fatalf("coverage-delta findings = %d, want 1: %+v", len(findings), findings)
	}
	finding := findings[0]
	if finding.Path != "calc.go" {
		t.Fatalf("finding path = %q, want calc.go", finding.Path)
	}
	if finding.Level != "warn" {
		t.Fatalf("finding level = %q, want warn", finding.Level)
	}
	if !strings.Contains(finding.Message, "changed-line coverage 0%") {
		t.Fatalf("unexpected message: %s", finding.Message)
	}
}

func TestCoverageDeltaFailUnderEscalatesLevel(t *testing.T) {
	if testing.Short() {
		t.Skip("runs go test in a fixture module")
	}
	dir := setupGoCoverageRepo(t)
	writeRepoFile(t, filepath.Join(dir, "calc.go"), `package covdemo

func Add(a, b int) int {
	return a + b
}

func Sub(a, b int) int {
	return a - b - 0
}
`)

	failUnder := 50
	cfg := coverageDeltaConfig(dir, "go")
	cfg.Checks.QualityRules.CoverageDelta.FailUnder = &failUnder
	report := runDiffScan(t, cfg)

	findings := coverageDeltaFindings(report)
	if len(findings) != 1 {
		t.Fatalf("coverage-delta findings = %d, want 1: %+v", len(findings), findings)
	}
	if findings[0].Level != "fail" {
		t.Fatalf("finding level = %q, want fail", findings[0].Level)
	}
}

func TestCoverageDeltaPassesWhenChangedLinesAreCovered(t *testing.T) {
	if testing.Short() {
		t.Skip("runs go test in a fixture module")
	}
	dir := setupGoCoverageRepo(t)
	// Only Add changes, and Add is exercised by the existing test.
	writeRepoFile(t, filepath.Join(dir, "calc.go"), `package covdemo

func Add(a, b int) int {
	return b + a
}

func Sub(a, b int) int {
	return a - b
}
`)

	report := runDiffScan(t, coverageDeltaConfig(dir, "go"))

	if findings := coverageDeltaFindings(report); len(findings) != 0 {
		t.Fatalf("expected no coverage-delta findings, got %+v", findings)
	}
}

func TestCoverageDeltaStaysDisabledByDefault(t *testing.T) {
	dir := setupGoCoverageRepo(t)
	writeRepoFile(t, filepath.Join(dir, "calc.go"), `package covdemo

func Add(a, b int) int {
	return a + b
}

func Sub(a, b int) int {
	return a - b - 0
}
`)

	cfg := coverageDeltaConfig(dir, "go")
	cfg.Checks.QualityRules.CoverageDelta.Enabled = nil // fall back to the default

	report := runDiffScan(t, cfg)

	if findings := coverageDeltaFindings(report); len(findings) != 0 {
		t.Fatalf("expected coverage-delta to be off by default, got %+v", findings)
	}
}

func TestCoverageDeltaParsesLcovReportForConfiguredLanguage(t *testing.T) {
	dir := t.TempDir()
	writeRepoFile(t, filepath.Join(dir, "app.ts"), `export function compute(): number {
  return 1;
}

export function unused(): number {
  return 2;
}
`)
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "initial")
	runGit(t, dir, "checkout", "-b", "feature")

	writeRepoFile(t, filepath.Join(dir, "app.ts"), `export function compute(): number {
  return 1 + 0;
}

export function unused(): number {
  return 2 + 0;
}
`)
	// Pretend a test runner produced this report: changed line 2 is covered,
	// changed line 6 is not.
	writeRepoFile(t, filepath.Join(dir, "coverage", "lcov.info"), `SF:app.ts
DA:1,1
DA:2,1
DA:5,0
DA:6,0
end_of_record
`)

	cfg := coverageDeltaConfig(dir, "typescript")
	cfg.Checks.QualityRules.CoverageDelta.LanguageCommands = map[string]codeguard.CoverageCommandConfig{
		"typescript": {
			Name:       "noop-coverage",
			Command:    "true",
			ReportPath: "coverage/lcov.info",
		},
	}

	report := runDiffScan(t, cfg)

	findings := coverageDeltaFindings(report)
	if len(findings) != 1 {
		t.Fatalf("coverage-delta findings = %d, want 1: %+v", len(findings), findings)
	}
	finding := findings[0]
	if finding.Path != "app.ts" {
		t.Fatalf("finding path = %q, want app.ts", finding.Path)
	}
	if !strings.Contains(finding.Message, "changed-line coverage 50%") {
		t.Fatalf("unexpected message: %s", finding.Message)
	}
	if !strings.Contains(finding.Message, "lines 6") {
		t.Fatalf("expected uncovered line list, got: %s", finding.Message)
	}
}
