package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func writeGoRebuildCascadeFixture(t *testing.T, dir string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/rebuild\n\ngo 1.23.0\n")
	writeFile(t, filepath.Join(dir, "shared", "shared.go"), "package shared\n\nfunc Value() int { return 1 }\n")
	writeFile(t, filepath.Join(dir, "mid", "mid.go"), "package mid\n\nimport \"example.com/rebuild/shared\"\n\nfunc Value() int { return shared.Value() }\n")
	writeFile(t, filepath.Join(dir, "alpha", "alpha.go"), "package alpha\n\nimport \"example.com/rebuild/shared\"\n\nfunc Value() int { return shared.Value() }\n")
	writeFile(t, filepath.Join(dir, "beta", "beta.go"), "package beta\n\nimport \"example.com/rebuild/shared\"\n\nfunc Value() int { return shared.Value() }\n")
	writeFile(t, filepath.Join(dir, "gamma", "gamma.go"), "package gamma\n\nimport \"example.com/rebuild/mid\"\n\nfunc Value() int { return mid.Value() }\n")
}

func TestPerformanceCheckWarnsForGoRebuildHotPackageAndAmplifier(t *testing.T) {
	dir := t.TempDir()
	writeGoRebuildCascadeFixture(t, dir)

	cfg := performanceConfig("performance-go-rebuild-cascade", dir, "go")
	cfg.Checks.PerformanceRules.HotPackageImporterThreshold = 2
	cfg.Checks.PerformanceRules.RebuildAmplifierThreshold = 3

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Performance", "performance.go.hot-package")
	assertFindingRulePresent(t, report, "Performance", "performance.go.rebuild-amplifier")
}

func TestPerformanceCheckSkipsGoRebuildCascadeBelowThreshold(t *testing.T) {
	dir := t.TempDir()
	writeGoRebuildCascadeFixture(t, dir)

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-go-rebuild-cascade-neg", dir, "go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Performance", "performance.go.hot-package")
	assertFindingRuleAbsent(t, report, "Performance", "performance.go.rebuild-amplifier")
}

func TestPerformanceCheckRebuildCascadeToggleOff(t *testing.T) {
	dir := t.TempDir()
	writeGoRebuildCascadeFixture(t, dir)

	off := false
	cfg := performanceConfig("performance-go-rebuild-cascade-off", dir, "go")
	cfg.Checks.PerformanceRules.HotPackageImporterThreshold = 2
	cfg.Checks.PerformanceRules.RebuildAmplifierThreshold = 3
	cfg.Checks.PerformanceRules.DetectRebuildCascade = &off

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Performance", "performance.go.hot-package")
	assertFindingRuleAbsent(t, report, "Performance", "performance.go.rebuild-amplifier")
}

func TestPerformanceCheckDiffModeScopesRebuildCascadeToChangedPackage(t *testing.T) {
	dir := t.TempDir()
	writeGoRebuildCascadeFixture(t, dir)
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "CodeGuard Test")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "base")

	writeFile(t, filepath.Join(dir, "shared", "shared.go"), "package shared\n\nfunc Value() int { return 2 }\n")

	cfg := performanceConfig("performance-go-rebuild-cascade-diff", dir, "go")
	cfg.Checks.PerformanceRules.HotPackageImporterThreshold = 2
	cfg.Checks.PerformanceRules.RebuildAmplifierThreshold = 3

	report, err := codeguard.RunWithOptions(context.Background(), cfg, codeguard.ScanOptions{
		Mode:    codeguard.ScanModeDiff,
		BaseRef: "main",
	})
	if err != nil {
		t.Fatalf("run diff: %v", err)
	}

	assertFindingRulePresent(t, report, "Performance", "performance.go.hot-package")
	assertFindingRulePresent(t, report, "Performance", "performance.go.rebuild-amplifier")
}

func TestPerformanceCheckDiffModeSkipsUnchangedHotPackages(t *testing.T) {
	dir := t.TempDir()
	writeGoRebuildCascadeFixture(t, dir)
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "CodeGuard Test")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "base")

	writeFile(t, filepath.Join(dir, "gamma", "gamma.go"), "package gamma\n\nimport \"example.com/rebuild/mid\"\n\nfunc Value() int { return mid.Value() + 1 }\n")

	cfg := performanceConfig("performance-go-rebuild-cascade-diff-neg", dir, "go")
	cfg.Checks.PerformanceRules.HotPackageImporterThreshold = 2
	cfg.Checks.PerformanceRules.RebuildAmplifierThreshold = 3

	report, err := codeguard.RunWithOptions(context.Background(), cfg, codeguard.ScanOptions{
		Mode:    codeguard.ScanModeDiff,
		BaseRef: "main",
	})
	if err != nil {
		t.Fatalf("run diff: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Performance", "performance.go.hot-package")
	assertFindingRuleAbsent(t, report, "Performance", "performance.go.rebuild-amplifier")
}
