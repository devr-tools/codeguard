package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func writeChangeImpactRepo(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "CodeGuard Test")
	writeFile(t, filepath.Join(dir, "app", "base.py"), "VALUE = 1\n")
	writeFile(t, filepath.Join(dir, "app", "mid.py"), "from app import base\n\nMID = base.VALUE\n")
	writeFile(t, filepath.Join(dir, "app", "top.py"), "from app import mid\n\nTOP = mid.MID\n")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "base")
	writeFile(t, filepath.Join(dir, "app", "base.py"), "VALUE = 2\n")
}

func changeImpactArtifact(t *testing.T, report codeguard.Report) *codeguard.ChangeImpactArtifact {
	t.Helper()
	for _, artifact := range report.Artifacts {
		if artifact.Kind == "change-impact" && artifact.ChangeImpact != nil {
			return artifact.ChangeImpact
		}
	}
	t.Fatal("change-impact artifact not found")
	return nil
}

func TestDiffModeEmitsChangeImpactArtifactAndHighImpactWarning(t *testing.T) {
	dir := t.TempDir()
	writeChangeImpactRepo(t, dir)

	cfg := graphTestConfig("design-change-impact", dir, "python")
	cfg.Checks.DesignRules.HighImpactChangeThreshold = 1

	report, err := codeguard.RunWithOptions(context.Background(), cfg, codeguard.ScanOptions{
		Mode:    codeguard.ScanModeDiff,
		BaseRef: "main",
	})
	if err != nil {
		t.Fatalf("run diff: %v", err)
	}

	assertFindingRulePresent(t, report, "Design Patterns", "design.high-impact-change")

	artifact := changeImpactArtifact(t, report)
	if artifact.BaseRef != "main" {
		t.Fatalf("artifact base ref = %q, want %q", artifact.BaseRef, "main")
	}
	for _, entry := range artifact.Entries {
		if entry.File != "app/base.py" {
			continue
		}
		if entry.Module != "app.base" {
			t.Fatalf("entry module = %q, want %q", entry.Module, "app.base")
		}
		if entry.TransitiveDependents != 2 {
			t.Fatalf("entry dependents = %d, want 2", entry.TransitiveDependents)
		}
		return
	}
	t.Fatalf("artifact missing entry for app/base.py: %+v", artifact.Entries)
}

func TestDiffModeSkipsHighImpactWarningBelowThreshold(t *testing.T) {
	dir := t.TempDir()
	writeChangeImpactRepo(t, dir)

	report, err := codeguard.RunWithOptions(context.Background(), graphTestConfig("design-change-impact-neg", dir, "python"), codeguard.ScanOptions{
		Mode:    codeguard.ScanModeDiff,
		BaseRef: "main",
	})
	if err != nil {
		t.Fatalf("run diff: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Design Patterns", "design.high-impact-change")
	if artifact := changeImpactArtifact(t, report); len(artifact.Entries) == 0 {
		t.Fatal("expected change-impact artifact entries below threshold")
	}
}

func TestFullScanEmitsNoChangeImpactArtifact(t *testing.T) {
	dir := t.TempDir()
	writeChangeImpactRepo(t, dir)

	report, err := codeguard.Run(context.Background(), graphTestConfig("design-change-impact-full", dir, "python"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	for _, artifact := range report.Artifacts {
		if artifact.Kind == "change-impact" {
			t.Fatal("full scan should not emit a change-impact artifact")
		}
	}
}
