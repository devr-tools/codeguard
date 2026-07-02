// Package corpus_test is a ground-truth precision/recall harness for the
// security detectors. It scans the fixture tree under testdata/ through the
// public SDK (one scan per language target) and checks every finding against
// tests/corpus/expectations.yaml: expected findings must fire (else FN),
// anything else on a fixture is a false positive, and documented known gaps
// are asserted to still exist so they get promoted when fixed.
package corpus_test

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// finding is one deduplicated in-scope scanner hit on a fixture file.
type finding struct {
	Rule string
	Line int
}

func TestCorpusExpectations(t *testing.T) {
	man := loadManifest(t)
	if t.Failed() {
		t.Fatal("expectations manifest is invalid")
	}
	assertManifestMatchesFixtures(t, man)
	if t.Failed() {
		t.Fatal("expectations manifest and fixture tree are out of sync")
	}

	inScope := ruleSet(man)
	board := newScoreboard(man.Rules)
	for _, group := range man.Groups {
		byFile := scanGroup(t, group, inScope)
		evaluateGroup(t, group, byFile, board)
	}
	t.Log("corpus precision/recall by rule:\n" + board.render())
}

// scanGroup runs one full security scan rooted at the group's fixture tree
// and returns the in-scope findings keyed by target-relative file path.
func scanGroup(t *testing.T, group fixtureGroup, inScope map[string]bool) map[string][]finding {
	t.Helper()
	if group.TreeSitter {
		defer forceTreeSitterScanPath(t)()
	}
	root, err := filepath.Abs(filepath.FromSlash(group.Root))
	if err != nil {
		t.Fatalf("group %s: resolve root %s: %v", group.Name, group.Root, err)
	}
	report, err := codeguard.RunWithOptions(context.Background(), groupConfig(group, root), codeguard.ScanOptions{Mode: codeguard.ScanModeFull})
	if err != nil {
		t.Fatalf("group %s: scan failed: %v", group.Name, err)
	}
	return collectFindings(t, group, report, inScope)
}

// forceTreeSitterScanPath points the TypeScript semantic-engine discovery at
// an existing but invalid lib for the duration of one group scan, so the
// analyzer errors and the target takes the per-file path — the tree-sitter
// path a treesitter group exists to measure. Without this, hosts where a
// real TypeScript lib is discoverable (e.g. via a VS Code install) would let
// the Node semantic engine claim the target and parsers.treesitter would
// never be exercised. The returned func restores the previous environment.
func forceTreeSitterScanPath(t *testing.T) func() {
	t.Helper()
	const libEnv = "CODEGUARD_TYPESCRIPT_LIB_PATH"
	bogus := filepath.Join(t.TempDir(), "not-typescript.js")
	if err := os.WriteFile(bogus, []byte("throw new Error('not a TypeScript lib');\n"), 0o600); err != nil {
		t.Fatalf("write bogus typescript lib: %v", err)
	}
	previous, hadPrevious := os.LookupEnv(libEnv)
	if err := os.Setenv(libEnv, bogus); err != nil {
		t.Fatalf("set %s: %v", libEnv, err)
	}
	return func() {
		if hadPrevious {
			_ = os.Setenv(libEnv, previous)
			return
		}
		_ = os.Unsetenv(libEnv)
	}
}

// groupConfig builds a security-only SDK config for one fixture group. The
// cache is disabled so scans never write state into the repository, and
// govulncheck is off because fixture trees are not real modules.
func groupConfig(group fixtureGroup, root string) codeguard.Config {
	enabled := true
	disabled := false
	secrets := &codeguard.SecretsRulesConfig{Enabled: &enabled}
	if group.Entropy {
		secrets.Entropy = &codeguard.SecretsEntropyConfig{Enabled: &enabled}
	}
	cfg := codeguard.Config{
		Name: "corpus-" + group.Name,
		Targets: []codeguard.TargetConfig{{
			Name:     group.Name,
			Path:     root,
			Language: group.Language,
		}},
		Checks: codeguard.CheckConfig{
			Security: true,
			SecurityRules: codeguard.SecurityRulesConfig{
				GovulncheckMode: "off",
				Secrets:         secrets,
			},
		},
		Output: codeguard.OutputConfig{Format: "text"},
		Cache:  codeguard.CacheConfig{Enabled: &disabled},
	}
	if group.TreeSitter {
		cfg.Parsers = codeguard.ParsersConfig{TreeSitter: "auto"}
	}
	return cfg
}

// collectFindings filters the report down to in-scope rules and deduplicates
// by (rule, path, line): repeated hits of one rule on one line (e.g. several
// taint chains reaching the same sink) count once against the manifest.
func collectFindings(t *testing.T, group fixtureGroup, report codeguard.Report, inScope map[string]bool) map[string][]finding {
	t.Helper()
	byFile := make(map[string][]finding)
	seen := make(map[string]map[finding]bool)
	for _, section := range report.Sections {
		for _, item := range section.Findings {
			if !inScope[item.RuleID] {
				continue
			}
			if item.Path == "" {
				t.Errorf("group %s: in-scope rule %s produced a finding with no path: %s", group.Name, item.RuleID, item.Message)
				continue
			}
			path := filepath.ToSlash(item.Path)
			hit := finding{Rule: item.RuleID, Line: item.Line}
			if seen[path] == nil {
				seen[path] = make(map[finding]bool)
			}
			if seen[path][hit] {
				continue
			}
			seen[path][hit] = true
			byFile[path] = append(byFile[path], hit)
		}
	}
	for path := range byFile {
		sort.Slice(byFile[path], func(i, j int) bool {
			if byFile[path][i].Line != byFile[path][j].Line {
				return byFile[path][i].Line < byFile[path][j].Line
			}
			return byFile[path][i].Rule < byFile[path][j].Rule
		})
	}
	return byFile
}
