package checks_test

import (
	"context"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// taintRuleLines returns the sorted sink lines a rule reported, so a test can
// pin exactly which call sites fire rather than only that something did.
func taintRuleLines(report codeguard.Report, ruleID string) []int {
	lines := make([]int, 0)
	for _, section := range report.Sections {
		for _, finding := range section.Findings {
			if finding.RuleID == ruleID {
				lines = append(lines, finding.Line)
			}
		}
	}
	sort.Ints(lines)
	return lines
}

func assertTaintLines(t *testing.T, report codeguard.Report, ruleID string, want ...int) {
	t.Helper()
	got := taintRuleLines(report, ruleID)
	if len(got) != len(want) {
		t.Fatalf("%s lines = %v, want %v; messages %v", ruleID, got, want, taintMessages(t, report, ruleID))
	}
	for idx := range want {
		if got[idx] != want[idx] {
			t.Fatalf("%s lines = %v, want %v; messages %v", ruleID, got, want, taintMessages(t, report, ruleID))
		}
	}
}

// A sink whose result is consumed by a chained method call
// (exec.Command(...).Run()) must be analyzed like the bound form
// (cmd := exec.Command(...)); the receiver of the outer call used to be
// skipped entirely.
func TestGoTaintChainedSinkCalls(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), strings.Join([]string{
		"package main",                         // 1
		"",                                     // 2
		"import (",                             // 3
		"\t\"net/http\"",                       // 4
		"\t\"os\"",                             // 5
		"\t\"os/exec\"",                        // 6
		"\t\"strconv\"",                        // 7
		")",                                    // 8
		"",                                     // 9
		"func chained() {",                     // 10
		"\t_ = exec.Command(os.Args[1]).Run()", // 11 tainted
		"}",                                    // 12
		"",                                     // 13
		"func inline(w http.ResponseWriter, r *http.Request) {",                // 14
		"\t_ = exec.Command(\"sh\", \"-c\", r.URL.Query().Get(\"x\")).Run()",   // 15 tainted
		"\tif err := exec.Command(r.FormValue(\"bin\")).Start(); err != nil {", // 16 tainted
		"\t\treturn", // 17
		"\t}",        // 18
		"\tout, _ := exec.Command(\"git\", \"status\").Output()", // 19 constant args
		"\t_ = out", // 20
		"\tn, _ := strconv.Atoi(r.URL.Query().Get(\"n\"))",     // 21 sanitized
		"\t_ = exec.Command(\"sleep\", strconv.Itoa(n)).Run()", // 22 sanitized
		"\tdefer exec.Command(os.Args[2]).Wait()",              // 23 tainted (defer)
		"}", // 24
		"",
	}, "\n"))

	report, err := codeguard.Run(context.Background(), securityOnlyConfig("taint-go-chained", dir, "go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertTaintLines(t, report, "security.taint.go", 11, 15, 16, 23)
	messages := taintMessages(t, report, "security.taint.go")
	assertChainMessage(t, messages, "os.Args", "exec.Command (line 11)")
	assertChainMessage(t, messages, "r.URL.Query().Get", "exec.Command (line 15)")
	assertFindingConfidence(t, report, "Security", "security.taint.go", "high")
}

// A chained call on a constant command built by a helper whose parameter
// reaches the sink must still resolve through the function summary.
func TestGoTaintChainedSinkInsideHelperResolvesAtCallSite(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), strings.Join([]string{
		"package main",                      // 1
		"",                                  // 2
		"import (",                          // 3
		"\t\"os\"",                          // 4
		"\t\"os/exec\"",                     // 5
		")",                                 // 6
		"",                                  // 7
		"func run(name string) error {",     // 8
		"\treturn exec.Command(name).Run()", // 9 sink for parameter
		"}",                                 // 10
		"",                                  // 11
		"func main() {",                     // 12
		"\t_ = run(\"ls\")",                 // 13 constant
		"\t_ = run(os.Getenv(\"TOOL\"))",    // 14 tainted
		"}",                                 // 15
		"",
	}, "\n"))

	report, err := codeguard.Run(context.Background(), securityOnlyConfig("taint-go-chained-helper", dir, "go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertTaintLines(t, report, "security.taint.go", 9)
	assertChainMessage(t, taintMessages(t, report, "security.taint.go"), "os.Getenv", "run()", "exec.Command (line 9)")
}
