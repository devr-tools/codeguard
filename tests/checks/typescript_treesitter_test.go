package checks_test

import (
	"context"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// The adversarial differential corpus for the tree-sitter migration
// (docs/treesitter-spike.md phase 2) lives in testdata/treesitter. Each
// fixture line may carry markers:
//
//	EXPECT <rule>       ground truth: the rule genuinely applies here
//	BASELINE-FN <rule>  the regex path misses this line
//	BASELINE-FP <rule>  the regex path wrongly flags this line
//
// With parsers.treesitter "auto" the scan must report exactly the EXPECT
// set; with "off" it must report exactly the pinned regex behavior
// (EXPECT - BASELINE-FN + BASELINE-FP), so both engines are locked at once.

// disableTypeScriptSemanticEngine forces the per-file scan path (regex or
// tree-sitter) by pointing the semantic-engine discovery at an existing but
// invalid TypeScript lib: the analyzer then errors and every target falls
// back, keeping these differential tests deterministic on machines where a
// real TypeScript lib is discoverable (e.g. via a VS Code install).
func disableTypeScriptSemanticEngine(t *testing.T) {
	t.Helper()
	bogus := filepath.Join(t.TempDir(), "not-typescript.js")
	writeFile(t, bogus, "throw new Error('not a TypeScript lib');\n")
	t.Setenv("CODEGUARD_TYPESCRIPT_LIB_PATH", bogus)
}

func treesitterScanConfig(root string, mode string) codeguard.Config {
	disabled := false
	cfg := codeguard.Config{
		Name:    "treesitter-differential",
		Targets: []codeguard.TargetConfig{{Name: "fixtures", Path: root, Language: "typescript"}},
		Checks:  codeguard.CheckConfig{Quality: true, Security: true},
		Output:  codeguard.OutputConfig{Format: "text"},
		Cache:   codeguard.CacheConfig{Enabled: &disabled},
	}
	if mode != "" {
		cfg.Parsers = codeguard.ParsersConfig{TreeSitter: mode}
	}
	return cfg
}

// scanTreesitterCorpus runs a scan and reduces it to the migrated rules'
// findings keyed by marker shape, also returning each finding's confidence.
func scanTreesitterCorpus(t *testing.T, root string, mode string) (map[treesitterFinding]bool, map[treesitterFinding]string) {
	t.Helper()
	report, err := codeguard.Run(context.Background(), treesitterScanConfig(root, mode))
	if err != nil {
		t.Fatalf("scan (mode %q): %v", mode, err)
	}
	found := map[treesitterFinding]bool{}
	confidence := map[treesitterFinding]string{}
	for _, section := range report.Sections {
		for _, item := range section.Findings {
			rule := migratedRuleToken(item.RuleID)
			if rule == "" {
				continue
			}
			finding := treesitterFinding{File: filepath.Base(item.Path), Rule: rule, Line: item.Line}
			found[finding] = true
			confidence[finding] = item.Confidence
		}
	}
	return found, confidence
}

// migratedRuleToken maps a full rule ID onto its marker token, accepting the
// typescript and javascript mirrors alike; other rules return "".
func migratedRuleToken(ruleID string) string {
	for _, family := range []string{"quality.typescript.", "quality.javascript.", "security.typescript.", "security.javascript."} {
		if suffix, ok := strings.CutPrefix(ruleID, family); ok && treesitterRuleTokens[suffix] {
			return suffix
		}
	}
	return ""
}

func sortedTreesitterFindings(set map[treesitterFinding]bool) []treesitterFinding {
	out := make([]treesitterFinding, 0, len(set))
	for finding := range set {
		out = append(out, finding)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		if out[i].Line != out[j].Line {
			return out[i].Line < out[j].Line
		}
		return out[i].Rule < out[j].Rule
	})
	return out
}

func assertTreesitterFindingSet(t *testing.T, label string, got map[treesitterFinding]bool, want map[treesitterFinding]bool) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s findings mismatch\n got: %v\nwant: %v", label, sortedTreesitterFindings(got), sortedTreesitterFindings(want))
	}
}

// TestTreesitterDifferentialCorpus locks both engines to the adversarial
// corpus: the tree path must report exactly the ground truth, and the regex
// path must keep its pinned false positives/negatives (so a regex
// improvement fails loudly here and gets the markers updated).
func TestTreesitterDifferentialCorpus(t *testing.T) {
	disableTypeScriptSemanticEngine(t)
	root := filepath.Join("testdata", "treesitter")
	markers := loadTreesitterMarkers(t, root)

	autoFindings, autoConfidence := scanTreesitterCorpus(t, root, "auto")
	assertTreesitterFindingSet(t, "parsers.treesitter=auto", autoFindings, markers.expect)
	for finding, level := range autoConfidence {
		if markers.expect[finding] && level != "high" {
			t.Errorf("tree-path finding %v confidence = %q, want high", finding, level)
		}
	}

	offFindings, offConfidence := scanTreesitterCorpus(t, root, "off")
	assertTreesitterFindingSet(t, "parsers.treesitter=off", offFindings, markers.baselineSet())
	for finding, level := range offConfidence {
		if level == "high" {
			t.Errorf("regex-path finding %v unexpectedly reports confidence high", finding)
		}
	}
}

// TestTreesitterOffMatchesDefault asserts that "off" is a byte-for-byte
// no-op: the default configuration (parsers unset) and an explicit "off"
// produce identical findings across every section.
func TestTreesitterOffMatchesDefault(t *testing.T) {
	disableTypeScriptSemanticEngine(t)
	root := filepath.Join("testdata", "treesitter")

	defaultReport, err := codeguard.Run(context.Background(), treesitterScanConfig(root, ""))
	if err != nil {
		t.Fatalf("scan (default): %v", err)
	}
	offReport, err := codeguard.Run(context.Background(), treesitterScanConfig(root, "off"))
	if err != nil {
		t.Fatalf("scan (off): %v", err)
	}
	if !reflect.DeepEqual(defaultReport.Sections, offReport.Sections) {
		t.Fatalf("explicit parsers.treesitter=off diverges from the default configuration\ndefault: %+v\noff: %+v", defaultReport.Sections, offReport.Sections)
	}
}
