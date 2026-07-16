package checks_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// initComplexityRepo builds a git repo whose main branch holds baseSource in
// main.go, then leaves the worktree on a feature branch so tests can write
// the changed revision and diff against main.
func initComplexityRepo(t *testing.T, baseSource string) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), baseSource)
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "CodeGuard Test")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "base")
	runGit(t, dir, "checkout", "-b", "feature")
	return dir
}

func complexityRegressionConfig(dir string) codeguard.Config {
	cfg := performanceConfig("performance-complexity-regression", dir, "go")
	cfg.Cache.Enabled = boolPtr(false)
	return cfg
}

func runPerformanceDiffScan(t *testing.T, cfg codeguard.Config) codeguard.Report {
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

func complexityRegressionFindings(report codeguard.Report) []codeguard.Finding {
	findings := make([]codeguard.Finding, 0)
	for _, section := range report.Sections {
		for _, finding := range section.Findings {
			if finding.RuleID == "performance.complexity-regression" {
				findings = append(findings, finding)
			}
		}
	}
	return findings
}

const (
	complexitySingleLoopBase = `package sample

func UpdateAll(values []int) int {
	total := 0
	for _, value := range values {
		total += value
	}
	return total
}
`

	complexityNestedLoopHead = `package sample

func UpdateAll(values []int) int {
	total := 0
	for _, value := range values {
		for step := 0; step < value; step++ {
			total += step
		}
	}
	return total
}
`
)

func TestComplexityRegressionWarnsOnNestingDepthIncrease(t *testing.T) {
	dir := initComplexityRepo(t, complexitySingleLoopBase)
	writeFile(t, filepath.Join(dir, "main.go"), complexityNestedLoopHead)

	report := runPerformanceDiffScan(t, complexityRegressionConfig(dir))

	findings := complexityRegressionFindings(report)
	if len(findings) != 1 {
		t.Fatalf("complexity-regression findings = %d, want 1: %+v", len(findings), findings)
	}
	finding := findings[0]
	if finding.Level != "warn" {
		t.Fatalf("finding level = %q, want warn", finding.Level)
	}
	if finding.Path != "main.go" {
		t.Fatalf("finding path = %q, want main.go", finding.Path)
	}
	want := "function UpdateAll: loop nesting depth increased from 1 to 2 in this change"
	if !strings.Contains(finding.Message, want) {
		t.Fatalf("message = %q, want it to contain %q", finding.Message, want)
	}
	if finding.Line <= 0 {
		t.Fatalf("finding line = %d, want a positive changed line", finding.Line)
	}
}

func TestComplexityRegressionMatchesMethodsByReceiver(t *testing.T) {
	dir := initComplexityRepo(t, `package sample

type Store struct{ items []int }

func (s *Store) UpdateAll() int {
	total := 0
	for _, item := range s.items {
		total += item
	}
	return total
}
`)
	writeFile(t, filepath.Join(dir, "main.go"), `package sample

type Store struct{ items []int }

func (s *Store) UpdateAll() int {
	total := 0
	for round := 0; round < 2; round++ {
		for _, item := range s.items {
			total += item * round
		}
	}
	return total
}
`)

	report := runPerformanceDiffScan(t, complexityRegressionConfig(dir))

	findings := complexityRegressionFindings(report)
	if len(findings) != 1 {
		t.Fatalf("complexity-regression findings = %d, want 1: %+v", len(findings), findings)
	}
	want := "function Store.UpdateAll: loop nesting depth increased from 1 to 2"
	if !strings.Contains(findings[0].Message, want) {
		t.Fatalf("message = %q, want it to contain %q", findings[0].Message, want)
	}
}

func TestComplexityRegressionSkipsFunctionsNotInBase(t *testing.T) {
	dir := initComplexityRepo(t, complexitySingleLoopBase)
	// A brand-new function with nested loops has no base version to regress
	// from; the existing function is untouched.
	writeFile(t, filepath.Join(dir, "main.go"), complexitySingleLoopBase+`
func Pairs(values []int) int {
	count := 0
	for _, left := range values {
		for _, right := range values {
			if left < right {
				count++
			}
		}
	}
	return count
}
`)

	report := runPerformanceDiffScan(t, complexityRegressionConfig(dir))

	if findings := complexityRegressionFindings(report); len(findings) != 0 {
		t.Fatalf("expected no complexity-regression findings for new functions, got %+v", findings)
	}
}

func TestComplexityRegressionSilentWhenDepthUnchanged(t *testing.T) {
	dir := initComplexityRepo(t, complexityNestedLoopHead)
	// Touch a line inside the nested loop without changing the nesting depth.
	writeFile(t, filepath.Join(dir, "main.go"), strings.Replace(
		complexityNestedLoopHead, "total += step", "total += step + 1", 1))

	report := runPerformanceDiffScan(t, complexityRegressionConfig(dir))

	if findings := complexityRegressionFindings(report); len(findings) != 0 {
		t.Fatalf("expected no complexity-regression findings when depth is unchanged, got %+v", findings)
	}
}

func TestComplexityRegressionSilentInFullScan(t *testing.T) {
	dir := initComplexityRepo(t, complexitySingleLoopBase)
	writeFile(t, filepath.Join(dir, "main.go"), complexityNestedLoopHead)

	report, err := codeguard.Run(context.Background(), complexityRegressionConfig(dir))
	if err != nil {
		t.Fatalf("full scan: %v", err)
	}

	if findings := complexityRegressionFindings(report); len(findings) != 0 {
		t.Fatalf("expected the rule to stay silent in full-scan mode, got %+v", findings)
	}
}

func TestComplexityRegressionToggleDisablesRule(t *testing.T) {
	dir := initComplexityRepo(t, complexitySingleLoopBase)
	writeFile(t, filepath.Join(dir, "main.go"), complexityNestedLoopHead)

	cfg := complexityRegressionConfig(dir)
	cfg.Checks.PerformanceRules.DetectComplexityRegression = boolPtr(false)
	report := runPerformanceDiffScan(t, cfg)

	if findings := complexityRegressionFindings(report); len(findings) != 0 {
		t.Fatalf("expected no findings with detect_complexity_regression disabled, got %+v", findings)
	}
}
