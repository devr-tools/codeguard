package codeguard_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// A patch is written against the repository root (svc/config.go) while a
// target may point at a subdirectory (svc). RunPatch must rebase the patch
// onto the target when scoping findings, not only when applying it, or every
// finding in such a target is filtered out and the patch passes.
func TestRunPatchScopesFindingsForTargetBelowRepositoryRoot(t *testing.T) {
	dir := t.TempDir()
	writeAPITestFile(t, filepath.Join(dir, "svc", "config.go"), "package main\n\nfunc Stable() {}\n")
	runAPITestGit(t, dir, "init", "-q", "-b", "main")

	cfg := codeguard.ExampleConfig()
	cfg.Targets = []codeguard.TargetConfig{{Name: "svc", Path: filepath.Join(dir, "svc"), Language: "go"}}
	cfg.Checks.Quality = false
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false

	secret := "ghp_" + "0123456789abcdefghijklmnopqrstuvwxyz"
	diff := strings.Join([]string{
		"diff --git a/svc/config.go b/svc/config.go",
		"--- a/svc/config.go",
		"+++ b/svc/config.go",
		"@@ -1,3 +1,4 @@",
		" package main",
		"",
		"+const token = \"" + secret + "\"",
		" func Stable() {}",
		"",
	}, "\n")

	report, err := codeguard.RunPatch(context.Background(), cfg, diff)
	if err != nil {
		t.Fatalf("run patch: %v", err)
	}
	for _, section := range report.Sections {
		for _, finding := range section.Findings {
			if finding.RuleID == "security.hardcoded-credential" && finding.Path == "config.go" && finding.Line == 3 {
				if report.Summary.FailedSections == 0 {
					t.Fatalf("credential found but patch did not fail: %#v", report.Summary)
				}
				return
			}
		}
	}
	t.Fatalf("expected a hardcoded credential at config.go:3 for the svc target, got summary %#v", report.Summary)
}
