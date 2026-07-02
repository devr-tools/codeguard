package corpus_test

import (
	"fmt"
	"strings"
	"testing"
)

// evaluateGroup checks one group's actual findings against its manifest
// entries, records per-rule tallies, and fails the test with a readable
// message for every deviation from the expected ground truth.
func evaluateGroup(t *testing.T, group fixtureGroup, byFile map[string][]finding, board *scoreboard) {
	t.Helper()
	for _, file := range group.Files {
		evaluateFile(t, group, file, board, byFile[file.Path])
		delete(byFile, file.Path)
	}
	// Guards against scanner path drift: the manifest/fixture sync check
	// already proved every scanned path has a manifest entry.
	for _, path := range sortedKeys(pathSet(byFile)) {
		for _, hit := range byFile[path] {
			board.add(hit.Rule, statFP)
			t.Errorf("group %s: finding on unlisted path %s: %s line %d", group.Name, path, hit.Rule, hit.Line)
		}
	}
}

// fileFindings tracks one fixture's actual findings and which of them have
// been accounted for by a must_fire or known_gaps entry.
type fileFindings struct {
	hits    []finding
	covered []bool
}

func newFileFindings(hits []finding) *fileFindings {
	return &fileFindings{hits: hits, covered: make([]bool, len(hits))}
}

// mark marks every finding satisfying (rule, line) as covered and reports
// whether at least one matched. Line 0 matches any line.
func (f *fileFindings) mark(rule string, line int) bool {
	matched := false
	for idx, hit := range f.hits {
		if hit.Rule != rule || (line != 0 && hit.Line != line) {
			continue
		}
		f.covered[idx] = true
		matched = true
	}
	return matched
}

func evaluateFile(t *testing.T, group fixtureGroup, file fixtureFile, board *scoreboard, hits []finding) {
	t.Helper()
	actual := newFileFindings(hits)
	name := group.Name + "/" + file.Path

	for _, exp := range file.MustFire {
		if actual.mark(exp.Rule, exp.Line) {
			board.add(exp.Rule, statTP)
			continue
		}
		board.add(exp.Rule, statFN)
		t.Errorf("%s: expected %s to fire at %s, but it did not; actual findings: %s",
			name, exp.Rule, describeLine(exp.Line), renderFindings(actual.hits))
	}
	for _, gap := range file.KnownGaps {
		evaluateKnownGap(t, name, gap, actual, board)
	}
	for idx, hit := range actual.hits {
		if actual.covered[idx] {
			continue
		}
		board.add(hit.Rule, statFP)
		t.Errorf("%s: unexpected finding (false positive): %s at line %d", name, hit.Rule, hit.Line)
	}
}

// evaluateKnownGap asserts that a documented gap still behaves as recorded:
// a known false negative must stay silent and a known false positive must
// still fire. Either way the gap is charged against the rule's metrics.
func evaluateKnownGap(t *testing.T, name string, gap knownGap, actual *fileFindings, board *scoreboard) {
	t.Helper()
	switch gap.Type {
	case gapFalseNegative:
		board.add(gap.Rule, statKnownFN)
		if actual.mark(gap.Rule, gap.Line) {
			t.Errorf("%s: known false-negative gap for %s at %s now fires — promote it to must_fire (%s)",
				name, gap.Rule, describeLine(gap.Line), gap.Reason)
		}
	case gapFalsePositive:
		board.add(gap.Rule, statKnownFP)
		if !actual.mark(gap.Rule, gap.Line) {
			t.Errorf("%s: known false-positive gap for %s at %s no longer fires — remove the known_gaps entry (%s)",
				name, gap.Rule, describeLine(gap.Line), gap.Reason)
		}
	}
}

func renderFindings(actual []finding) string {
	if len(actual) == 0 {
		return "(none)"
	}
	parts := make([]string, 0, len(actual))
	for _, hit := range actual {
		parts = append(parts, fmt.Sprintf("%s@%d", hit.Rule, hit.Line))
	}
	return strings.Join(parts, ", ")
}

func pathSet(byFile map[string][]finding) map[string]bool {
	set := make(map[string]bool, len(byFile))
	for path := range byFile {
		set[path] = true
	}
	return set
}
