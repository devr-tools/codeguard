package treesitter

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// corpusMarkers is the ground truth parsed from the marker comments in
// testdata/adversarial.ts (see the header of that file for the grammar).
type corpusMarkers struct {
	expect      []Finding // what a correct implementation must report
	baselineFPs []Finding // extra findings the current regexes produce
	baselineFNs []Finding // real findings the current regexes miss
}

func readCorpus(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read corpus %s: %v", name, err)
	}
	return data
}

func parseMarkers(t testing.TB, source []byte) corpusMarkers {
	t.Helper()
	markers := corpusMarkers{}
	for idx, line := range strings.Split(string(source), "\n") {
		tokens := strings.Fields(line)
		for pos := 0; pos+1 < len(tokens); pos++ {
			rule := tokens[pos+1]
			if rule != RuleExplicitAny && rule != RuleUnsafeHTMLSink {
				continue // e.g. the marker-grammar description in the header
			}
			finding := Finding{Rule: rule, Line: idx + 1}
			switch tokens[pos] {
			case "EXPECT":
				markers.expect = append(markers.expect, finding)
			case "BASELINE-FP":
				markers.baselineFPs = append(markers.baselineFPs, finding)
			case "BASELINE-FN":
				markers.baselineFNs = append(markers.baselineFNs, finding)
			}
		}
	}
	if len(markers.expect) == 0 {
		t.Fatal("corpus has no EXPECT markers; marker parsing is broken")
	}
	return markers
}

func findingSet(findings []Finding) map[Finding]struct{} {
	set := make(map[Finding]struct{}, len(findings))
	for _, f := range findings {
		set[f] = struct{}{}
	}
	return set
}

// TestEnginesMatchGroundTruth: every tree-sitter engine must report exactly
// the EXPECT-marked findings on the adversarial corpus - no false
// positives, no false negatives.
func TestEnginesMatchGroundTruth(t *testing.T) {
	source := readCorpus(t, "adversarial.ts")
	want := normalizeFindings(parseMarkers(t, source).expect)
	for _, engine := range engines {
		t.Run(engine.Name(), func(t *testing.T) {
			got, err := engine.Scan(source)
			if err != nil {
				t.Fatalf("scan: %v", err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("findings mismatch\n got: %v\nwant: %v", got, want)
			}
		})
	}
}

// TestBaselinePrecisionGap pins the exact false-positive/false-negative
// behavior of the current regex implementation on the adversarial corpus,
// so the numbers quoted in docs/treesitter-spike.md are enforced by CI of
// this spike module.
func TestBaselinePrecisionGap(t *testing.T) {
	source := readCorpus(t, "adversarial.ts")
	markers := parseMarkers(t, source)

	got := BaselineScan(source)
	truth := findingSet(markers.expect)
	misses := findingSet(markers.baselineFNs)

	wantBaseline := make([]Finding, 0, len(markers.expect))
	for _, f := range markers.expect {
		if _, missed := misses[f]; !missed {
			wantBaseline = append(wantBaseline, f)
		}
	}
	wantBaseline = normalizeFindings(append(wantBaseline, markers.baselineFPs...))
	if !reflect.DeepEqual(got, wantBaseline) {
		t.Fatalf("baseline behavior drifted from markers\n got: %v\nwant: %v", got, wantBaseline)
	}

	truePositives := 0
	for _, f := range got {
		if _, ok := truth[f]; ok {
			truePositives++
		}
	}
	precision := float64(truePositives) / float64(len(got))
	recall := float64(truePositives) / float64(len(markers.expect))
	t.Logf("adversarial corpus: ground truth=%d findings", len(markers.expect))
	t.Logf("baseline regex: reported=%d tp=%d fp=%d fn=%d precision=%.1f%% recall=%.1f%%",
		len(got), truePositives, len(markers.baselineFPs), len(markers.baselineFNs),
		100*precision, 100*recall)
	t.Logf("tree-sitter engines: precision=100.0%% recall=100.0%% (TestEnginesMatchGroundTruth)")
}

// TestRealisticParity checks two things on everyday (non-adversarial) code:
// the pure-Go and CGo tree-sitter runtimes agree exactly, and the current
// regexes agree with them too - i.e. the precision gap is specifically
// about the constructs in the adversarial corpus, not everyday code.
func TestRealisticParity(t *testing.T) {
	source := readCorpus(t, "realistic.ts")
	baseline := BaselineScan(source)
	if len(baseline) == 0 {
		t.Fatal("realistic corpus should contain some findings")
	}
	for _, engine := range engines {
		t.Run(engine.Name(), func(t *testing.T) {
			got, err := engine.Scan(source)
			if err != nil {
				t.Fatalf("scan: %v", err)
			}
			if !reflect.DeepEqual(got, baseline) {
				t.Errorf("engine disagrees with baseline on realistic corpus\n got: %v\nbaseline: %v", got, baseline)
			}
		})
	}
	t.Logf("realistic corpus: %d findings, identical across baseline and %d engine(s)", len(baseline), len(engines))
}
