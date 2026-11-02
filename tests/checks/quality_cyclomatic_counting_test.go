package checks_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// cyclomaticCountingFixture has one function whose Go complexity differs by
// counting mode: legacy counts the if, three case clauses, and the default
// clause (1+1+3 = 5) and ignores the three short-circuit operators; gocyclo
// counts the if, the two non-default cases, and the && / || operators
// (1+1+2+3 = 7) and ignores default.
const cyclomaticCountingFixture = `package main

func route(a, b, c, d bool, kind int) int {
	if a && b || c && d {
		return 0
	}
	switch kind {
	case 1:
		return 1
	case 2:
		return 2
	default:
		return 3
	}
}
`

func runCyclomaticCountingScan(t *testing.T, mode string, threshold int) codeguard.Report {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), cyclomaticCountingFixture)

	cfg := codeguard.ExampleConfig()
	cfg.Name = "quality-cyclomatic-counting"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Quality = true
	cfg.Checks.QualityRules.MaxCyclomaticComplexity = threshold
	cfg.Checks.QualityRules.CyclomaticCounting = mode
	cfg.Checks.Security = false
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run (cyclomatic_counting=%q): %v", mode, err)
	}
	return report
}

func cyclomaticFindingMessage(t *testing.T, report codeguard.Report) string {
	t.Helper()
	findings := findingsForRule(report, "quality.cyclomatic-complexity")
	if len(findings) != 1 {
		t.Fatalf("quality.cyclomatic-complexity findings = %+v, want exactly one", findings)
	}
	return findings[0].Message
}

// TestCyclomaticCountingModesReportDifferentGoValues pins the per-mode Go
// value end to end through the quality section's finding message.
func TestCyclomaticCountingModesReportDifferentGoValues(t *testing.T) {
	for _, tc := range []struct {
		mode string
		want string
	}{
		{"", "function route has cyclomatic complexity 5; max is 1"},
		{"legacy", "function route has cyclomatic complexity 5; max is 1"},
		{"gocyclo", "function route has cyclomatic complexity 7; max is 1"},
	} {
		t.Run("mode="+tc.mode, func(t *testing.T) {
			if got := cyclomaticFindingMessage(t, runCyclomaticCountingScan(t, tc.mode, 1)); got != tc.want {
				t.Fatalf("message = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestCyclomaticCountingDefaultIsLegacy is the 1.x compatibility guarantee: a
// config that does not set cyclomatic_counting keeps the pre-gocyclo Go
// count, so a function at the threshold under legacy counting does not
// newly warn after a minor upgrade, while opting into gocyclo does.
func TestCyclomaticCountingDefaultIsLegacy(t *testing.T) {
	if got := findingsForRule(runCyclomaticCountingScan(t, "", 5), "quality.cyclomatic-complexity"); len(got) != 0 {
		t.Fatalf("default counting reported %+v at max 5; the 1.x default must be legacy (complexity 5)", got)
	}
	if got := codeguard.ExampleConfig().Checks.QualityRules.CyclomaticCounting; got != "" && got != "legacy" {
		t.Fatalf("example config cyclomatic_counting = %q, want unset or legacy", got)
	}
	message := cyclomaticFindingMessage(t, runCyclomaticCountingScan(t, "gocyclo", 5))
	if !strings.Contains(message, "complexity 7") {
		t.Fatalf("gocyclo message = %q, want complexity 7", message)
	}
}
