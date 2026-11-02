package checks_test

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// The quality section runs its whole-target passes (per-file language rules,
// clone detection, AI target analysis, ...) concurrently and fans their
// per-file work out to worker pools. Findings must still come out in one fixed
// order, so two scans of the same tree must agree finding for finding.
func TestQualityParallelPassesProduceIdenticalFindingOrder(t *testing.T) {
	dir := writeParallelQualityFixture(t)
	first := qualityFindingSequence(t, dir)
	if len(first) == 0 {
		t.Fatal("fixture produced no quality findings; the determinism check would be vacuous")
	}
	assertParallelFixtureCoverage(t, first)
	for run := 2; run <= 4; run++ {
		next := qualityFindingSequence(t, dir)
		if len(next) != len(first) {
			t.Fatalf("run %d produced %d findings, run 1 produced %d", run, len(next), len(first))
		}
		for i := range first {
			if next[i] != first[i] {
				t.Fatalf("run %d finding %d = %q, run 1 had %q", run, i, next[i], first[i])
			}
		}
	}
}

// writeParallelQualityFixture writes more Go files than one worker batch holds,
// spread over several packages, with duplicated function bodies (clone
// detection), unused private helpers in every package (cross-file dead code),
// swallowed errors and narrative comments (per-file AI rules).
func writeParallelQualityFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/parallel\n\ngo 1.22\n")
	const packages = 6
	const filesPerPackage = 60
	for pkg := 0; pkg < packages; pkg++ {
		for file := 0; file < filesPerPackage; file++ {
			path := filepath.Join(dir, fmt.Sprintf("pkg%d", pkg), fmt.Sprintf("file%03d.go", file))
			writeFile(t, path, parallelFixtureSource(pkg, file))
		}
	}
	return dir
}

func parallelFixtureSource(pkg int, file int) string {
	lines := []string{
		fmt.Sprintf("package pkg%d", pkg),
		"",
		`import "strconv"`,
		"",
		fmt.Sprintf("// Compute%d adds one to the value and then returns it.", file),
		fmt.Sprintf("func Compute%d(value string) int {", file),
		"\tparsed, err := strconv.Atoi(value)",
		"\t_ = err",
		"\ttotal := parsed + 1",
		"\tif total%2 == 0 {",
		"\t\ttotal = total * 3",
		"\t}",
		"\tfor total < 20 {",
		"\t\ttotal = total + 2",
		"\t}",
		"\tif total > 25 {",
		"\t\treturn total - 4",
		"\t}",
		"\treturn total + 5",
		"}",
		"",
	}
	if file%7 == 0 {
		lines = append(lines,
			fmt.Sprintf("func unusedHelper%d(input int) int {", file),
			"\treturn input * 2",
			"}",
			"",
		)
	}
	return strings.Join(lines, "\n")
}

func qualityFindingSequence(t *testing.T, dir string) []string {
	t.Helper()
	cfg := codeguard.ExampleConfig()
	cfg.Name = "quality-parallel-determinism"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Quality = true
	cfg.Checks.Design = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.QualityRules.CloneTokenThreshold = 20
	disabled := false
	cfg.Cache.Enabled = &disabled

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	var sequence []string
	for _, section := range report.Sections {
		if section.Name != "Code Quality" {
			continue
		}
		for _, finding := range section.Findings {
			sequence = append(sequence, fmt.Sprintf("%s|%s:%d:%d|%s", finding.RuleID, finding.Path, finding.Line, finding.Column, finding.Message))
		}
	}
	return sequence
}

// assertParallelFixtureCoverage checks the fixture exercises every pass whose
// ordering the test is meant to pin down.
func assertParallelFixtureCoverage(t *testing.T, sequence []string) {
	t.Helper()
	for _, rule := range []string{"quality.duplicate-code", "quality.ai.swallowed-error", "quality.ai.dead-code"} {
		found := false
		for _, entry := range sequence {
			if strings.HasPrefix(entry, rule+"|") {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("fixture produced no %s finding", rule)
		}
	}
}
