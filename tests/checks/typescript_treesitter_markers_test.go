package checks_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// treesitterRuleTokens maps marker tokens onto the rule-ID suffix shared by
// the quality.typescript/javascript and security.typescript/javascript
// families.
var treesitterRuleTokens = map[string]bool{
	"explicit-any":       true,
	"non-null-assertion": true,
	"double-assertion":   true,
	"unsafe-html-sink":   true,
}

type treesitterFinding struct {
	File string
	Rule string // marker token (rule-ID suffix)
	Line int
}

type treesitterMarkers struct {
	expect      map[treesitterFinding]bool
	baselineFNs map[treesitterFinding]bool
	baselineFPs map[treesitterFinding]bool
}

func loadTreesitterMarkers(t *testing.T, dir string) treesitterMarkers {
	t.Helper()
	markers := treesitterMarkers{
		expect:      map[treesitterFinding]bool{},
		baselineFNs: map[treesitterFinding]bool{},
		baselineFPs: map[treesitterFinding]bool{},
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read corpus dir: %v", err)
	}
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatalf("read corpus %s: %v", entry.Name(), err)
		}
		for idx, line := range strings.Split(string(data), "\n") {
			markers.recordLineMarkers(entry.Name(), idx+1, line)
		}
	}
	if len(markers.expect) == 0 {
		t.Fatal("corpus has no EXPECT markers; marker parsing is broken")
	}
	return markers
}

// recordLineMarkers scans one fixture line for marker/rule token pairs.
func (m treesitterMarkers) recordLineMarkers(file string, lineNo int, line string) {
	tokens := strings.Fields(line)
	for pos := 0; pos+1 < len(tokens); pos++ {
		rule := tokens[pos+1]
		if !treesitterRuleTokens[rule] {
			continue
		}
		finding := treesitterFinding{File: file, Rule: rule, Line: lineNo}
		switch tokens[pos] {
		case "EXPECT":
			m.expect[finding] = true
		case "BASELINE-FN":
			m.baselineFNs[finding] = true
		case "BASELINE-FP":
			m.baselineFPs[finding] = true
		}
	}
}

// baselineSet derives the pinned regex behavior from the markers.
func (m treesitterMarkers) baselineSet() map[treesitterFinding]bool {
	set := make(map[treesitterFinding]bool, len(m.expect))
	for finding := range m.expect {
		if !m.baselineFNs[finding] {
			set[finding] = true
		}
	}
	for finding := range m.baselineFPs {
		set[finding] = true
	}
	return set
}
