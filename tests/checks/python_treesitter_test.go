package checks_test

import (
	"context"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// The Python N+1 differential tests pin both engines for
// performance.n-plus-one-query: the tree-sitter path (parsers.treesitter
// "auto") must report only real query calls inside loops, while the regex
// path ("off" — the fallback lever for the tree path, since Python has no
// Node semantic engine to disable) keeps its pinned false positives on
// query-shaped text inside comments and string literals.

const pythonNPlusOneFixture = `import requests


def fetch_rows(items, cursor):
    results = []
    for item in items:
        row = cursor.execute("SELECT name FROM users WHERE id = ?")
        results.append(row)
    return results


def build_batch_notes(items):
    notes = []
    for item in items:
        # batching hint: cursor.execute(BULK_SQL) once supported
        notes.append("cursor.execute(%s)" % item)
    return notes


def poll(url):
    while True:
        response = requests.get(url)
        return response
`

// Fixture line numbers (1-based).
const (
	pythonNPlusOneExecuteLine = 7  // true positive: cursor.execute inside for
	pythonNPlusOneCommentLine = 15 // regex FP: query text inside a comment
	pythonNPlusOneStringLine  = 16 // regex FP: query text inside a string literal
	pythonNPlusOneGetLine     = 22 // true positive: requests.get inside while
)

const pythonNPlusOneWantMessage = "query or request call inside a loop suggests an N+1 pattern; batch the work or hoist the call out of the loop"

func pythonTreesitterConfig(dir string, mode string) codeguard.Config {
	cfg := performanceConfig("python-treesitter-differential", dir, "python")
	disabled := false
	cfg.Cache = codeguard.CacheConfig{Enabled: &disabled}
	if mode != "" {
		cfg.Parsers = codeguard.ParsersConfig{TreeSitter: mode}
	}
	return cfg
}

// scanPythonNPlusOne returns the performance.n-plus-one-query finding lines
// plus each line's confidence, asserting the shared rule ID/message contract
// along the way.
func scanPythonNPlusOne(t *testing.T, dir string, mode string) ([]int, map[int]string) {
	t.Helper()
	report, err := codeguard.Run(context.Background(), pythonTreesitterConfig(dir, mode))
	if err != nil {
		t.Fatalf("scan (mode %q): %v", mode, err)
	}
	lines := make([]int, 0, 4)
	confidence := map[int]string{}
	for _, section := range report.Sections {
		for _, finding := range section.Findings {
			if finding.RuleID != "performance.n-plus-one-query" {
				continue
			}
			if finding.Message != pythonNPlusOneWantMessage {
				t.Errorf("mode %q line %d message = %q, want %q", mode, finding.Line, finding.Message, pythonNPlusOneWantMessage)
			}
			lines = append(lines, finding.Line)
			confidence[finding.Line] = finding.Confidence
		}
	}
	sort.Ints(lines)
	return lines, confidence
}

// TestPythonNPlusOneTreeDifferential proves the tree path is strictly more
// precise than the regex fallback on the same fixture: both report the two
// genuine query calls in loops, and only the regex path also flags the
// query-shaped comment and string-literal lines.
func TestPythonNPlusOneTreeDifferential(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "service.py"), pythonNPlusOneFixture)

	truePositives := []int{pythonNPlusOneExecuteLine, pythonNPlusOneGetLine}

	treeLines, treeConfidence := scanPythonNPlusOne(t, dir, "auto")
	if !reflect.DeepEqual(treeLines, truePositives) {
		t.Errorf("tree path lines = %v, want exactly the true positives %v", treeLines, truePositives)
	}
	for line, level := range treeConfidence {
		if level != "high" {
			t.Errorf("tree path line %d confidence = %q, want high", line, level)
		}
	}

	regexLines, regexConfidence := scanPythonNPlusOne(t, dir, "off")
	wantRegex := []int{pythonNPlusOneExecuteLine, pythonNPlusOneCommentLine, pythonNPlusOneStringLine, pythonNPlusOneGetLine}
	sort.Ints(wantRegex)
	if !reflect.DeepEqual(regexLines, wantRegex) {
		t.Errorf("regex path lines = %v, want true positives plus pinned comment/string FPs %v", regexLines, wantRegex)
	}
	for line, level := range regexConfidence {
		if level == "high" {
			t.Errorf("regex path line %d unexpectedly reports confidence high", line)
		}
	}
}

// TestPythonNPlusOneTreeOffMatchesDefault locks the fallback lever: leaving
// parsers unset must behave exactly like an explicit "off" for Python
// targets, so the tree path stays opt-in.
func TestPythonNPlusOneTreeOffMatchesDefault(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "service.py"), pythonNPlusOneFixture)

	defaultReport, err := codeguard.Run(context.Background(), pythonTreesitterConfig(dir, ""))
	if err != nil {
		t.Fatalf("scan (default): %v", err)
	}
	offReport, err := codeguard.Run(context.Background(), pythonTreesitterConfig(dir, "off"))
	if err != nil {
		t.Fatalf("scan (off): %v", err)
	}
	if !reflect.DeepEqual(defaultReport.Sections, offReport.Sections) {
		t.Fatalf("explicit parsers.treesitter=off diverges from the default configuration\ndefault: %+v\noff: %+v", defaultReport.Sections, offReport.Sections)
	}
}
