package checks_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func budgetConfig(name string, dir string, budgets []codeguard.PerformanceBudgetConfig) codeguard.Config {
	cfg := performanceConfig(name, dir, "go")
	cfg.Checks.PerformanceRules.Budgets = budgets
	return cfg
}

// findBudgetFindingMessage returns the first performance.budget finding
// message containing want, failing the test when none matches.
func findBudgetFindingMessage(t *testing.T, report codeguard.Report, want string) {
	t.Helper()
	for _, section := range report.Sections {
		if section.Name != "Performance" {
			continue
		}
		for _, finding := range section.Findings {
			if finding.RuleID == "performance.budget" && strings.Contains(finding.Message, want) {
				return
			}
		}
	}
	t.Fatalf("no performance.budget finding containing %q", want)
}

func TestPerformanceBudgetFileSizeUnderBudgetPasses(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "dist", "app.bin"), strings.Repeat("x", 100))

	report, err := codeguard.Run(context.Background(), budgetConfig("budget-under", dir, []codeguard.PerformanceBudgetConfig{
		{Name: "binary", Kind: "file-size", Path: "dist/app.bin", MaxBytes: 200},
	}))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertSectionStatus(t, report, "Performance", "pass")
	assertFindingRuleAbsent(t, report, "Performance", "performance.budget")
}

func TestPerformanceBudgetFileSizeOverBudgetFailLevel(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "dist", "app.bin"), strings.Repeat("x", 500))

	report, err := codeguard.Run(context.Background(), budgetConfig("budget-over-fail", dir, []codeguard.PerformanceBudgetConfig{
		{Name: "binary", Kind: "file-size", Path: "dist/app.bin", MaxBytes: 100, Level: "fail"},
	}))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertSectionStatus(t, report, "Performance", "fail")
	assertFindingRulePresent(t, report, "Performance", "performance.budget")
	findBudgetFindingMessage(t, report, "totals 500 bytes")
}

func TestPerformanceBudgetGlobSumsMatches(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "dist", "a.js"), strings.Repeat("a", 300))
	writeFile(t, filepath.Join(dir, "dist", "b.js"), strings.Repeat("b", 300))

	report, err := codeguard.Run(context.Background(), budgetConfig("budget-glob", dir, []codeguard.PerformanceBudgetConfig{
		{Name: "bundles", Kind: "file-size", Path: "dist/*.js", MaxBytes: 500},
	}))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertSectionStatus(t, report, "Performance", "warn")
	findBudgetFindingMessage(t, report, "totals 600 bytes")
}

func TestPerformanceBudgetMissingArtifactWarnsOnly(t *testing.T) {
	dir := t.TempDir()

	// level fail must not apply to a missing artifact: absence is a warn-level
	// diagnostic, never a hard failure (dist/ may simply not be built here).
	report, err := codeguard.Run(context.Background(), budgetConfig("budget-missing", dir, []codeguard.PerformanceBudgetConfig{
		{Name: "binary", Kind: "file-size", Path: "dist/app.bin", MaxBytes: 100, Level: "fail"},
	}))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertSectionStatus(t, report, "Performance", "warn")
	findBudgetFindingMessage(t, report, "not found")
}

// TestPerformanceBudgetRelativeTargetPath guards a real containment bug: with
// a relative target path (the repository self-scan uses "."), EvalSymlinks
// keeps matches relative, and comparing them against the absolute canonical
// root falsely reported every artifact as escaping.
func TestPerformanceBudgetRelativeTargetPath(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "dist", "app.bin"), strings.Repeat("x", 300))
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	relDir, err := filepath.Rel(cwd, dir)
	if err != nil {
		t.Skipf("temp dir not relativizable from %s: %v", cwd, err)
	}

	report, err := codeguard.Run(context.Background(), budgetConfig("budget-relative-target", relDir, []codeguard.PerformanceBudgetConfig{
		{Name: "binary", Kind: "file-size", Path: "dist/app.bin", MaxBytes: 100},
	}))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertSectionStatus(t, report, "Performance", "warn")
	findBudgetFindingMessage(t, report, "totals 300 bytes")
}

func TestPerformanceBudgetPathEscapeRejectedByValidation(t *testing.T) {
	dir := t.TempDir()
	cfg := budgetConfig("budget-escape", dir, []codeguard.PerformanceBudgetConfig{
		{Name: "escape", Kind: "file-size", Path: "../outside.bin", MaxBytes: 100},
	})
	if err := codeguard.ValidateConfig(cfg); err == nil || !strings.Contains(err.Error(), "..") {
		t.Fatalf("expected validation error for escaping budget path, got %v", err)
	}
}

func TestPerformanceBudgetValidationRejectsBadEntries(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		label  string
		budget codeguard.PerformanceBudgetConfig
		want   string
	}{
		{"empty name", codeguard.PerformanceBudgetConfig{Kind: "file-size", Path: "a", MaxBytes: 1}, "name is required"},
		{"unknown kind", codeguard.PerformanceBudgetConfig{Name: "x", Kind: "zip-size", Path: "a", MaxBytes: 1}, "kind must be"},
		{"non-positive max_bytes", codeguard.PerformanceBudgetConfig{Name: "x", Kind: "file-size", Path: "a", MaxBytes: 0}, "max_bytes must be positive"},
		{"absolute path", codeguard.PerformanceBudgetConfig{Name: "x", Kind: "file-size", Path: "/etc/passwd", MaxBytes: 1}, "must be relative"},
		{"asset on file-size", codeguard.PerformanceBudgetConfig{Name: "x", Kind: "file-size", Path: "a", Asset: "b", MaxBytes: 1}, "asset only applies"},
		{"bad level", codeguard.PerformanceBudgetConfig{Name: "x", Kind: "file-size", Path: "a", MaxBytes: 1, Level: "error"}, "level must be"},
	}
	for _, tc := range cases {
		cfg := budgetConfig("budget-validate", dir, []codeguard.PerformanceBudgetConfig{tc.budget})
		err := codeguard.ValidateConfig(cfg)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: expected error containing %q, got %v", tc.label, tc.want, err)
		}
	}
}

func TestPerformanceBudgetSymlinkEscapeRejected(t *testing.T) {
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "big.bin"), strings.Repeat("x", 1000))
	dir := t.TempDir()
	if err := os.Symlink(filepath.Join(outside, "big.bin"), filepath.Join(dir, "app.bin")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	report, err := codeguard.Run(context.Background(), budgetConfig("budget-symlink-escape", dir, []codeguard.PerformanceBudgetConfig{
		{Name: "binary", Kind: "file-size", Path: "app.bin", MaxBytes: 1, Level: "fail"},
	}))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	// The symlinked artifact resolves outside the target: the budget is skipped
	// with a warn diagnostic instead of measuring (or failing on) the foreign file.
	assertSectionStatus(t, report, "Performance", "warn")
	findBudgetFindingMessage(t, report, "resolves outside the target directory")
}
