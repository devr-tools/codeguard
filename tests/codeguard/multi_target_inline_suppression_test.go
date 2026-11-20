package codeguard_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// Findings carry only a target-relative path, so with targets a/ and b/ that
// both hold main.go the finding's own file is ambiguous. An inline ignore in
// a/main.go must not hide b/main.go's credential, which would turn a failing
// gate into a passing one; a directive present in both files still applies.
func TestInlineIgnoreDoesNotLeakAcrossTargetsWithTheSameRelativePath(t *testing.T) {
	secret := "\"" + "AKIA" + "Z7Q3XK2LMNPRSTUV" + "\""
	clean := "package main\n\nfunc main() {}\n\n// placeholder\n"
	leaky := "package main\n\nfunc main() {}\n\nconst awsKey = " + secret + "\n"
	ignoreLine := "// codeguard:ignore security.hardcoded-credential -- fixture\n"
	ignoredClean := "package main\n\nfunc main() {}\n" + ignoreLine + "// placeholder\n"
	ignoredLeaky := "package main\n\nfunc main() {}\n" + ignoreLine + "const awsKey = " + secret + "\n"

	cases := []struct {
		name      string
		a, b      string
		wantFound bool
	}{
		{name: "ignore only in the other target", a: ignoredClean, b: leaky, wantFound: true},
		{name: "ignore in both targets", a: ignoredClean, b: ignoredLeaky, wantFound: false},
		{name: "no ignore anywhere", a: clean, b: leaky, wantFound: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeRepoFile(t, filepath.Join(dir, "a", "main.go"), tc.a)
			writeRepoFile(t, filepath.Join(dir, "b", "main.go"), tc.b)

			cfg := codeguard.ExampleConfig()
			cfg.Targets = []codeguard.TargetConfig{
				{Name: "a", Path: filepath.Join(dir, "a"), Language: "go"},
				{Name: "b", Path: filepath.Join(dir, "b"), Language: "go"},
			}
			cfg.Checks.Quality = false
			cfg.Checks.Design = false
			cfg.Checks.Prompts = false
			cfg.Checks.CI = false

			report, err := codeguard.Run(context.Background(), cfg)
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			found := false
			for _, section := range report.Sections {
				for _, finding := range section.Findings {
					if finding.RuleID == "security.hardcoded-credential" && finding.Path == "main.go" {
						found = true
					}
				}
			}
			if found != tc.wantFound {
				t.Fatalf("credential reported = %v, want %v (summary %#v)", found, tc.wantFound, report.Summary)
			}
		})
	}
}
