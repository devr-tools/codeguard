package checks_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func gitCommit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(cmd.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestScanGitHistoryFindsRemovedSecret(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	gitCommit(t, dir, "init", "-q")

	// Commit a secret, then remove it in a later commit. A working-tree scan of
	// HEAD would miss it; the history scan must still find it.
	writeFile(t, filepath.Join(dir, "config.go"), "package main\nconst awsKey = \""+cred("AKIA", "1234567890ABCDEF")+"\"\n")
	gitCommit(t, dir, "add", ".")
	gitCommit(t, dir, "commit", "-q", "-m", "add config")

	writeFile(t, filepath.Join(dir, "config.go"), "package main\n// key removed\n")
	gitCommit(t, dir, "add", ".")
	gitCommit(t, dir, "commit", "-q", "-m", "remove key")

	report, err := codeguard.ScanGitHistory(context.Background(), codeguard.ExampleConfig(), codeguard.HistoryScanOptions{RepoPath: dir})
	if err != nil {
		t.Fatalf("scan history: %v", err)
	}

	var found bool
	for _, finding := range report.Findings {
		if finding.RuleID == "security.hardcoded-credential" && finding.Path == "config.go" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected hardcoded-credential in history, got %+v", report.Findings)
	}
	if report.CommitsScanned != 2 {
		t.Fatalf("commits scanned = %d, want 2", report.CommitsScanned)
	}
}

func TestScanGitHistoryRespectsAllowPaths(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	gitCommit(t, dir, "init", "-q")
	writeFile(t, filepath.Join(dir, "testdata", "fixture.go"), "package fixture\nconst k = \""+cred("AKIA", "1234567890ABCDEF")+"\"\n")
	gitCommit(t, dir, "add", ".")
	gitCommit(t, dir, "commit", "-q", "-m", "fixture")

	cfg := codeguard.ExampleConfig()
	cfg.Checks.SecurityRules.Secrets = &codeguard.SecretsRulesConfig{
		Enabled:    boolPtr(true),
		AllowPaths: []string{"testdata/**"},
	}

	report, err := codeguard.ScanGitHistory(context.Background(), cfg, codeguard.HistoryScanOptions{RepoPath: dir})
	if err != nil {
		t.Fatalf("scan history: %v", err)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("expected no findings under allow_paths, got %+v", report.Findings)
	}
}
