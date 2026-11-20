package codeguard_test

import (
	"context"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// Git writes a changed file whose name has a space as "+++ b/sp ace.go\t" and
// one with non-ASCII bytes C-quoted ("+++ \"b/caf\303\251.go\""). Diff mode
// must still scope those files, or a PR can slip a finding past the gate just
// by naming the file unusually.
func TestDiffScanKeepsFindingsInFilesWithSpacesAndNonASCIINames(t *testing.T) {
	dir := t.TempDir()
	writeRepoFile(t, filepath.Join(dir, "go.mod"), "module example.com/quoted\n\ngo 1.23.0\n")
	writeRepoFile(t, filepath.Join(dir, "base.go"), "package main\n")
	runGit(t, dir, "init", "-q", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-q", "-m", "initial")
	runGit(t, dir, "checkout", "-q", "-b", "feature")

	source := "package main\n\nconst awsKey = \"" + "AKIA" + "Z7Q3XK2LMNPRSTUV" + "\"\n"
	names := []string{"plain.go", "sp ace.go", "café.go"}
	for _, name := range names {
		writeRepoFile(t, filepath.Join(dir, name), source)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-q", "-m", "add files")

	cfg := codeguard.ExampleConfig()
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Quality = false
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false

	report, err := codeguard.RunWithOptions(context.Background(), cfg, codeguard.ScanOptions{
		Mode:    codeguard.ScanModeDiff,
		BaseRef: "main",
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	var got []string
	for _, section := range report.Sections {
		for _, finding := range section.Findings {
			if finding.RuleID == "security.hardcoded-credential" {
				got = append(got, finding.Path)
			}
		}
	}
	sort.Strings(got)
	want := append([]string(nil), names...)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("diff-mode credential findings in %v, want %v", got, want)
	}
}

func TestChangedFilesFromUnifiedDiffDecodesGitHeaderPaths(t *testing.T) {
	diff := "diff --git a/sp ace.go b/sp ace.go\n" +
		"--- a/sp ace.go\t\n" +
		"+++ b/sp ace.go\t\n" +
		"@@ -1 +1 @@\n-a\n+b\n" +
		"diff --git \"a/caf\\303\\251.go\" \"b/caf\\303\\251.go\"\n" +
		"--- \"a/caf\\303\\251.go\"\n" +
		"+++ \"b/caf\\303\\251.go\"\n" +
		"@@ -1 +1 @@\n-a\n+b\n" +
		"diff --git a/gone.go b/gone.go\n" +
		"--- a/gone.go\n" +
		"+++ /dev/null\n" +
		"@@ -1 +0,0 @@\n-a\n"
	got := runnersupport.ChangedFilesFromUnifiedDiff(diff)
	want := []string{"café.go", "sp ace.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("changed files = %q, want %q", got, want)
	}
}
