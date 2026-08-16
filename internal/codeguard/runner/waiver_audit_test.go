package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/internal/version"
)

func TestRunWithOptionsWaiverAuditArtifact(t *testing.T) {
	dir := t.TempDir()
	writeWaiverAuditSecret(t, filepath.Join(dir, "secrets.go"))

	report, err := RunWithOptions(context.Background(), core.Config{
		Name: "waiver-audit",
		Targets: []core.TargetConfig{{
			Name:     "repo",
			Path:     dir,
			Language: "go",
		}},
		Checks: core.CheckConfig{
			Security: true,
		},
		Cache: core.CacheConfig{
			Enabled: boolPtr(false),
		},
		Waivers: []core.WaiverConfig{
			{Rule: "security.hardcoded-secret", Path: "secrets.go", Reason: "known test secret"},
			{Rule: "security.hardcoded-secret", Path: "missing.go", Reason: "old false positive"},
			{Rule: "security.not-a-real-rule", Reason: "renamed rule"},
		},
	}, core.ScanOptions{Mode: core.ScanModeFull, EnableWaiverAudit: true})
	if err != nil {
		t.Fatal(err)
	}

	audit := requireWaiverAuditArtifact(t, report)
	active := requireWaiverAuditEntry(t, audit, 0, core.WaiverAuditStatusActive, 1)
	if len(active.MatchedFingerprints) == 0 {
		t.Fatalf("active waiver missing fingerprint evidence: %#v", active)
	}
	requireWaiverAuditEntry(t, audit, 1, core.WaiverAuditStatusUnused, 0)
	requireWaiverAuditEntry(t, audit, 2, core.WaiverAuditStatusUnknownRule, 0)
}

func TestRunWithOptionsWaiverAuditEdgeCases(t *testing.T) {
	dir := t.TempDir()
	writeWaiverAuditSecret(t, filepath.Join(dir, "pkg", "secrets.go"))

	report, err := RunWithOptions(context.Background(), core.Config{
		Name: "waiver-audit-edges",
		Targets: []core.TargetConfig{{
			Name:     "repo",
			Path:     dir,
			Language: "go",
		}},
		Checks: core.CheckConfig{
			Security: true,
		},
		Cache: core.CacheConfig{
			Enabled: boolPtr(false),
		},
		Waivers: []core.WaiverConfig{
			{Rule: "*", Path: "pkg/**", Reason: "broad migration waiver"},
			{Rule: "security.hardcoded-secret", Path: "pkg/*.go", Reason: "specific false positive"},
			{Rule: "security.hardcoded-secret", Path: "pkg/*.go", ExpiresOn: "2020-01-01"},
			{Rule: "*", Path: "docs/**"},
		},
	}, core.ScanOptions{Mode: core.ScanModeFull, EnableWaiverAudit: true})
	if err != nil {
		t.Fatal(err)
	}

	audit := requireWaiverAuditArtifact(t, report)
	requireWaiverAuditEntry(t, audit, 0, core.WaiverAuditStatusActive, 1)
	requireWaiverAuditEntry(t, audit, 1, core.WaiverAuditStatusActive, 1)
	requireWaiverAuditEntry(t, audit, 2, core.WaiverAuditStatusExpired, 0)
	requireWaiverAuditEntry(t, audit, 3, core.WaiverAuditStatusUnused, 0)
}

func TestRunWithOptionsWaiverAuditArtifactOmittedWhenDisabled(t *testing.T) {
	dir := t.TempDir()
	writeWaiverAuditSecret(t, filepath.Join(dir, "secrets.go"))

	report, err := RunWithOptions(context.Background(), core.Config{
		Name: "waiver-audit-disabled",
		Targets: []core.TargetConfig{{
			Name:     "repo",
			Path:     dir,
			Language: "go",
		}},
		Checks: core.CheckConfig{
			Security: true,
		},
		Cache:   core.CacheConfig{Enabled: boolPtr(false)},
		Waivers: []core.WaiverConfig{{Rule: "security.hardcoded-secret"}},
	}, core.ScanOptions{Mode: core.ScanModeFull})
	if err != nil {
		t.Fatal(err)
	}
	for _, artifact := range report.Artifacts {
		if artifact.WaiverAudit != nil {
			t.Fatalf("waiver audit artifact should be omitted unless enabled: %#v", artifact)
		}
	}
}

func TestRunWithOptionsWaiverAuditMarksStaleAfterVersionUpgrade(t *testing.T) {
	originalVersion := version.Number
	t.Cleanup(func() { version.Number = originalVersion })

	dir := t.TempDir()
	secretPath := filepath.Join(dir, "secrets.go")
	writeWaiverAuditSecret(t, secretPath)
	cfg := waiverAuditHistoryConfig(dir)

	version.Number = "1.0.0"
	first, err := RunWithOptions(context.Background(), cfg, core.ScanOptions{Mode: core.ScanModeFull, EnableWaiverAudit: true})
	if err != nil {
		t.Fatal(err)
	}
	requireWaiverAuditEntry(t, requireWaiverAuditArtifact(t, first), 0, core.WaiverAuditStatusActive, 1)

	writeWaiverAuditCleanFile(t, secretPath)
	version.Number = "1.1.0"
	second, err := RunWithOptions(context.Background(), cfg, core.ScanOptions{Mode: core.ScanModeFull, EnableWaiverAudit: true})
	if err != nil {
		t.Fatal(err)
	}
	entry := requireWaiverAuditEntry(t, requireWaiverAuditArtifact(t, second), 0, core.WaiverAuditStatusUnused, 0)
	if entry.UpgradeStatus != core.WaiverAuditUpgradeStatusStaleAfterUpgrade ||
		entry.PreviousVersion != "1.0.0" ||
		entry.PreviousMatches != 1 {
		t.Fatalf("entry = %#v, want stale-after-upgrade evidence from 1.0.0", entry)
	}
	if entry.UpgradeReason == "" {
		t.Fatalf("expected upgrade reason on stale waiver: %#v", entry)
	}
}

func TestRunWithOptionsWaiverAuditMarksUpgradeEvidenceInconclusiveWhenScopeChanges(t *testing.T) {
	originalVersion := version.Number
	t.Cleanup(func() { version.Number = originalVersion })

	dir := t.TempDir()
	secretPath := filepath.Join(dir, "secrets.go")
	writeWaiverAuditSecret(t, secretPath)
	cfg := waiverAuditHistoryConfig(dir)

	version.Number = "1.0.0"
	if _, err := RunWithOptions(context.Background(), cfg, core.ScanOptions{Mode: core.ScanModeFull, EnableWaiverAudit: true}); err != nil {
		t.Fatal(err)
	}

	writeWaiverAuditCleanFile(t, secretPath)
	version.Number = "1.1.0"
	report, err := RunWithOptions(context.Background(), cfg, core.ScanOptions{Mode: core.ScanModeFull, TargetPath: dir, EnableWaiverAudit: true})
	if err != nil {
		t.Fatal(err)
	}
	entry := requireWaiverAuditEntry(t, requireWaiverAuditArtifact(t, report), 0, core.WaiverAuditStatusUnused, 0)
	if entry.UpgradeStatus != core.WaiverAuditUpgradeStatusInconclusive {
		t.Fatalf("entry = %#v, want inconclusive upgrade evidence", entry)
	}
}

func waiverAuditHistoryConfig(dir string) core.Config {
	return core.Config{
		Name: "waiver-audit-history",
		Targets: []core.TargetConfig{{
			Name:     "repo",
			Path:     dir,
			Language: "go",
		}},
		Checks: core.CheckConfig{
			Security: true,
		},
		Cache: core.CacheConfig{
			Path: filepath.Join(dir, "cache", "scan.json"),
		},
		Waivers: []core.WaiverConfig{{Rule: "security.hardcoded-secret", Path: "secrets.go"}},
	}
}

func writeWaiverAuditSecret(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`package main

func main() {
	api_key := "Zx9Qw3Rt7Yu1Io5P"
	_ = api_key
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeWaiverAuditCleanFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func boolPtr(value bool) *bool {
	return &value
}

func requireWaiverAuditArtifact(t *testing.T, report core.Report) *core.WaiverAuditArtifact {
	t.Helper()
	for _, artifact := range report.Artifacts {
		if artifact.WaiverAudit != nil {
			return artifact.WaiverAudit
		}
	}
	t.Fatalf("waiver audit artifact missing: %#v", report.Artifacts)
	return nil
}

func requireWaiverAuditEntry(t *testing.T, audit *core.WaiverAuditArtifact, index int, status string, matches int) core.WaiverAuditEntry {
	t.Helper()
	if len(audit.Waivers) <= index {
		t.Fatalf("waiver index %d missing: %#v", index, audit.Waivers)
	}
	entry := audit.Waivers[index]
	if entry.Index != index || entry.Status != status || entry.MatchedFindings != matches {
		t.Fatalf("entry[%d] = %#v, want status %s and %d matches", index, entry, status, matches)
	}
	return entry
}
