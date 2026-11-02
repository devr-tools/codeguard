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

// gosecJustificationSource pins every directive shape the translation must
// accept or reject. Line numbers matter: the assertions below name them.
var gosecJustificationSource = strings.Join([]string{
	"package main", // 1
	"",             // 2
	"import (",     // 3
	"\t\"os\"",     // 4
	"\t\"os/exec\"",
	")", // 6
	"",
	"func run(name string) {", // 8
	"\t_ = exec.Command(name).Run() //nolint:gosec // name is a validated tool path",                  // 9: suppressed
	"\t_ = exec.Command(name).Run() //nolint:gosec",                                                   // 10: no justification
	"\t_ = exec.Command(name).Run() //nolint:gosec // ...",                                            // 11: punctuation is not a reason
	"\t//nolint:gosec // a standalone directive covers the next statement",                            // 12
	"\t_ = exec.Command(name).Run()",                                                                  // 13: suppressed
	"\t// #nosec G204 -- operator-supplied binary",                                                    // 14
	"\t_ = exec.Command(name).Run()",                                                                  // 15: suppressed
	"\t_ = exec.Command(name).Run() // #nosec G304 -- file inclusion is not the subprocess rule",      // 16: wrong gosec rule
	"\t_ = exec.Command(name).Run() //nolint:errcheck // a different linter",                          // 17: not gosec
	"\t_ = exec.Command(name).Run() //nolint:errcheck,gosec // gosec listed alongside another linter", // 18: suppressed
	"\tcmd := exec.Command(os.Args[1]) //nolint:gosec // taint is never covered",                      // 19: shell suppressed, taint kept
	"\t_ = exec.Command(name).Run()",                                                                  // 20: no directive
	"\t_ = cmd.Run()",
	"}",
	"",
}, "\n")

func TestSecurityHonorsJustifiedGosecDirectivesForShellExecutionOnly(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), gosecJustificationSource)

	report, err := codeguard.RunWithOptions(context.Background(), securityOnlyConfig("gosec-justification", dir, "go"), codeguard.ScanOptions{Mode: codeguard.ScanModeFull, IncludeSuppressed: true})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if got, want := findingLines(findingsForRule(report, "security.shell-execution")), []int{10, 11, 16, 17, 20}; !reflect.DeepEqual(got, want) {
		t.Fatalf("reported security.shell-execution lines = %v, want %v", got, want)
	}

	suppressed := make([]codeguard.Finding, 0)
	for _, finding := range report.SuppressedFindings {
		if finding.RuleID != "security.shell-execution" {
			continue
		}
		if finding.Suppression == nil || finding.Suppression.Kind != "inline" || finding.Suppression.Match != "gosec-justification" {
			t.Fatalf("line %d suppression = %+v, want inline gosec-justification", finding.Line, finding.Suppression)
		}
		suppressed = append(suppressed, finding)
	}
	if got, want := findingLines(suppressed), []int{9, 13, 15, 18, 19}; !reflect.DeepEqual(got, want) {
		t.Fatalf("suppressed security.shell-execution lines = %v, want %v", got, want)
	}

	// A linter-wide nolint:gosec says nothing about which risk was reviewed,
	// so the taint finding on the same line must still be reported.
	if got := findingLines(findingsForRule(report, "security.taint.go")); !reflect.DeepEqual(got, []int{19}) {
		t.Fatalf("reported security.taint.go lines = %v, want [19]", got)
	}
}

func TestSecurityIgnoresGosecDirectivesOutsideGoSource(t *testing.T) {
	dir := t.TempDir()
	// The Go line-scan fallback still runs for a non-Go file, but a gosec
	// directive there is not a Go review record and must not suppress.
	writeFile(t, filepath.Join(dir, "notes.txt"), "exec.Command(name) //nolint:gosec // looks justified\n")

	report := runSecurity(t, "gosec-non-go", dir, "go")
	if got := findingLines(findingsForRule(report, "security.shell-execution")); !reflect.DeepEqual(got, []int{1}) {
		t.Fatalf("reported security.shell-execution lines = %v, want [1]", got)
	}
}

func findingLines(findings []codeguard.Finding) []int {
	lines := make([]int, 0, len(findings))
	for _, finding := range findings {
		lines = append(lines, finding.Line)
	}
	sort.Ints(lines)
	return lines
}
