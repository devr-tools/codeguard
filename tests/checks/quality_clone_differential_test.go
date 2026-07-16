package checks_test

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// cloneDifferentialFixture is deliberately nontrivial: a three-way clone
// across files with differing identifier case (Total/total/ToTal) exercises
// case normalization, the long duplicated region exercises overlapping-window
// merging, delta.go's short duplicate stays below the threshold, and the
// unicode comment in alpha.go keeps byte-offset handling honest.
var cloneDifferentialFixture = map[string][]string{
	"alpha.go": {
		"package sample",
		"",
		"func alphaOne(items []int) int {",
		"\tTotal := 0",
		"\tfor _, item := range items {",
		"\t\tif item%3 == 0 {",
		"\t\t\tTotal += item * 2",
		"\t\t} else {",
		"\t\t\tTotal -= item",
		"\t\t}",
		"\t}",
		"\tif Total < 0 {",
		"\t\tTotal = -Total",
		"\t}",
		"\treturn Total",
		"}",
		"",
		"// café ünïcode ✓ comment keeps byte offsets honest",
		"func alphaTwo(name string) string {",
		"\tif name == \"\" {",
		"\t\treturn \"unknown\"",
		"\t}",
		"\treturn name + \"-alpha\"",
		"}",
		"",
	},
	"beta.go": {
		"package sample",
		"",
		"func betaOne(items []int) int {",
		"\ttotal := 0",
		"\tfor _, item := range items {",
		"\t\tif item%3 == 0 {",
		"\t\t\ttotal += item * 2",
		"\t\t} else {",
		"\t\t\ttotal -= item",
		"\t\t}",
		"\t}",
		"\tif total < 0 {",
		"\t\ttotal = -total",
		"\t}",
		"\treturn total",
		"}",
		"",
	},
	"gamma.go": {
		"package sample",
		"",
		"func gammaOne(items []int) int {",
		"\tToTal := 0",
		"\tfor _, item := range items {",
		"\t\tif item%3 == 0 {",
		"\t\t\tToTal += item * 2",
		"\t\t} else {",
		"\t\t\tToTal -= item",
		"\t\t}",
		"\t}",
		"\tif ToTal < 0 {",
		"\t\tToTal = -ToTal",
		"\t}",
		"\treturn ToTal",
		"}",
		"",
		"func gammaTwo(label string) string {",
		"\tif label == \"\" {",
		"\t\treturn \"unknown\"",
		"\t}",
		"\treturn label + \"-gamma\"",
		"}",
		"",
	},
	"delta.go": {
		"package sample",
		"",
		"func deltaOne(count int) int {",
		"\tif count < 0 {",
		"\t\tcount = -count",
		"\t}",
		"\treturn count",
		"}",
		"",
	},
}

// cloneDifferentialExpected was captured verbatim from the pre-optimization
// clone detector (fresh FNV-1a over every token's bytes per window, lowercased
// token values) running on cloneDifferentialFixture; do not regenerate it with
// the current algorithm.
var cloneDifferentialExpected = []string{
	"alpha.go:3 duplicate normalized token sequence of 56 tokens also found in beta.go:3 (threshold 25)",
	"alpha.go:3 duplicate normalized token sequence of 56 tokens also found in gamma.go:3 (threshold 25)",
	"beta.go:3 duplicate normalized token sequence of 56 tokens also found in alpha.go:3 (threshold 25)",
	"beta.go:3 duplicate normalized token sequence of 56 tokens also found in gamma.go:3 (threshold 25)",
	"gamma.go:3 duplicate normalized token sequence of 56 tokens also found in alpha.go:3 (threshold 25)",
	"gamma.go:3 duplicate normalized token sequence of 56 tokens also found in beta.go:3 (threshold 25)",
}

// TestQualityCloneFindingsMatchPreOptimizationBaseline is a differential test
// for the clone-detector hashing rewrite (per-token hashes + rolling window
// hash): the optimized detector must reproduce the captured pre-optimization
// findings byte for byte.
func TestQualityCloneFindingsMatchPreOptimizationBaseline(t *testing.T) {
	dir := t.TempDir()
	for name, lines := range cloneDifferentialFixture {
		writeFile(t, filepath.Join(dir, name), strings.Join(lines, "\n"))
	}

	cfg := codeguard.ExampleConfig()
	cfg.Name = "quality-clone-differential"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Quality = true
	cfg.Checks.Design = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.QualityRules.CloneTokenThreshold = 25
	cfg.Checks.QualityRules.MaxFunctionLines = 100
	cfg.Checks.QualityRules.MaxParameters = 10
	cfg.Checks.QualityRules.MaxCyclomaticComplexity = 20

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	got := make([]string, 0)
	for _, section := range report.Sections {
		for _, finding := range section.Findings {
			if finding.RuleID != "quality.duplicate-code" {
				continue
			}
			got = append(got, fmt.Sprintf("%s:%d %s", finding.Path, finding.Line, finding.Message))
		}
	}
	sort.Strings(got)

	if len(got) != len(cloneDifferentialExpected) {
		t.Fatalf("duplicate-code findings = %d, want %d:\n%s", len(got), len(cloneDifferentialExpected), strings.Join(got, "\n"))
	}
	for i, want := range cloneDifferentialExpected {
		if got[i] != want {
			t.Fatalf("finding %d = %q, want %q", i, got[i], want)
		}
	}
}
