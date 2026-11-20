package codeguard_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// Targets a/ and b/ both hold main.go; only b's has a credential, on line 5.
// Findings report the target-relative path main.go either way, so every later
// lookup must resolve against the finding's own target: its inline
// directives, the source its fingerprints hash, and its target's diff scope.

const multiTargetSecretLine = "const awsKey = \"" + "AKIA" + "Z7Q3XK2LMNPRSTUV" + "\"\n"

func multiTargetSource(line5 string) string {
	return "package main\n\nfunc main() {}\n\n" + line5
}

func multiTargetConfig(dir string) codeguard.Config {
	cfg := codeguard.ExampleConfig()
	cfg.Targets = []codeguard.TargetConfig{
		{Name: "a", Path: filepath.Join(dir, "a"), Language: "go"},
		{Name: "b", Path: filepath.Join(dir, "b"), Language: "go"},
	}
	cfg.Checks.Quality = false
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Cache.Path = filepath.Join(dir, ".cache", "cache.json")
	return cfg
}

func credentialFindingsAt(report codeguard.Report, path string, line int) int {
	count := 0
	for _, section := range report.Sections {
		for _, finding := range section.Findings {
			if finding.RuleID == "security.hardcoded-credential" && finding.Path == path && finding.Line == line {
				count++
			}
		}
	}
	return count
}

// A directive in the finding's own file suppresses it, with no copy needed in
// the other target's file of the same name.
func TestInlineIgnoreInTheFindingsOwnTargetSuppressesIt(t *testing.T) {
	dir := t.TempDir()
	writeRepoFile(t, filepath.Join(dir, "a", "main.go"), multiTargetSource("// placeholder\n"))
	writeRepoFile(t, filepath.Join(dir, "b", "main.go"),
		"package main\n\nfunc main() {}\n// codeguard:ignore security.hardcoded-credential -- fixture\n"+multiTargetSecretLine)

	report, err := codeguard.Run(context.Background(), multiTargetConfig(dir))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := credentialFindingsAt(report, "main.go", 5); got != 0 {
		t.Fatalf("credential reported %d times, want it suppressed by b/main.go's own directive (summary %#v)", got, report.Summary)
	}
	if report.Summary.SuppressedFindings == 0 {
		t.Fatalf("expected the credential to be counted as suppressed: %#v", report.Summary)
	}
}

// A finding's fingerprints hash its own target's source: b/main.go's
// credential gets exactly the fingerprints a scan of b alone gives it, not
// ones derived from a/main.go, the first target holding that path. Otherwise
// an edit to a/main.go would change b's baseline identity, and identical
// lines in the two files could match each other's baseline entries.
func TestFingerprintsHashTheFindingsOwnTargetSource(t *testing.T) {
	dir := t.TempDir()
	writeRepoFile(t, filepath.Join(dir, "a", "main.go"), multiTargetSource("// placeholder\n"))
	writeRepoFile(t, filepath.Join(dir, "b", "main.go"), multiTargetSource(multiTargetSecretLine))

	both := multiTargetConfig(dir)
	onlyB := multiTargetConfig(dir)
	onlyB.Targets = onlyB.Targets[1:]
	onlyB.Cache.Path = filepath.Join(dir, ".cache-b", "cache.json")

	want := credentialFinding(t, onlyB)
	got := credentialFinding(t, both)
	if got.Fingerprint != want.Fingerprint || got.ContextFingerprint != want.ContextFingerprint || got.ContentFingerprint != want.ContentFingerprint {
		t.Fatalf("multi-target fingerprints %s/%s/%s differ from b alone %s/%s/%s",
			got.Fingerprint[:8], got.ContextFingerprint[:8], got.ContentFingerprint[:8],
			want.Fingerprint[:8], want.ContextFingerprint[:8], want.ContentFingerprint[:8])
	}
}

func credentialFinding(t *testing.T, cfg codeguard.Config) codeguard.Finding {
	t.Helper()
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, section := range report.Sections {
		for _, finding := range section.Findings {
			if finding.RuleID == "security.hardcoded-credential" && finding.Path == "main.go" && finding.Line == 5 {
				return finding
			}
		}
	}
	t.Fatalf("no credential finding at main.go:5 (summary %#v)", report.Summary)
	return codeguard.Finding{}
}

// Diff mode scopes each finding to its own target's changes: editing line 5
// of a/main.go must not report b/main.go's unchanged credential on line 5.
func TestDiffScopeDoesNotConflateTargetsWithTheSameRelativePath(t *testing.T) {
	dir := t.TempDir()
	writeRepoFile(t, filepath.Join(dir, "a", "main.go"), multiTargetSource("// placeholder\n"))
	writeRepoFile(t, filepath.Join(dir, "b", "main.go"), multiTargetSource(multiTargetSecretLine))
	runGit(t, dir, "init", "-q", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-q", "-m", "initial")
	runGit(t, dir, "checkout", "-q", "-b", "feature")
	writeRepoFile(t, filepath.Join(dir, "a", "main.go"), multiTargetSource("// edited in a only\n"))
	runGit(t, dir, "commit", "-q", "-am", "edit a")

	report, err := codeguard.RunWithOptions(context.Background(), multiTargetConfig(dir), codeguard.ScanOptions{
		Mode:    codeguard.ScanModeDiff,
		BaseRef: "main",
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := credentialFindingsAt(report, "main.go", 5); got != 0 {
		t.Fatalf("unchanged credential in b/main.go reported %d times for a change to a/main.go", got)
	}
}
