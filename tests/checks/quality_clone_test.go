package checks_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestQualityCheckWarnsForDuplicateCodeAtConfiguredThreshold(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "alpha.go"), strings.Join([]string{
		"package sample",
		"",
		"func alpha(value int) int {",
		"\ttotal := value + 1",
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
	}, "\n"))
	writeFile(t, filepath.Join(dir, "beta.go"), strings.Join([]string{
		"package sample",
		"",
		"func beta(value int) int {",
		"\ttotal := value + 1",
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
	}, "\n"))

	cfg := codeguard.ExampleConfig()
	cfg.Name = "quality-clone-threshold"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Quality = true
	cfg.Checks.Design = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.QualityRules.CloneTokenThreshold = 20
	cfg.Checks.QualityRules.MaxFunctionLines = 100
	cfg.Checks.QualityRules.MaxParameters = 10
	cfg.Checks.QualityRules.MaxCyclomaticComplexity = 20

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Code Quality", "warn")
	assertFindingRulePresent(t, report, "Code Quality", "quality.duplicate-code")
}

func TestQualityCheckUsesProfileAwareCloneThreshold(t *testing.T) {
	dir := t.TempDir()
	body := []string{
		"package sample",
		"",
		"func alpha(value int) int {",
		"\ttotal := value + 1",
		"\tif total%2 == 0 {",
		"\t\ttotal = total * 3",
		"\t}",
		"\tfor total < 20 {",
		"\t\ttotal = total + 2",
		"\t}",
		"\tif total > 25 {",
		"\t\treturn total - 4",
		"\t}",
		"\tif total < 3 {",
		"\t\ttotal++",
		"\t}",
		"\treturn total + 5",
		"}",
		"",
	}
	writeFile(t, filepath.Join(dir, "alpha.go"), strings.Join(body, "\n"))
	writeFile(t, filepath.Join(dir, "beta.go"), strings.Join(body, "\n"))

	cfg, err := codeguard.ExampleConfigForProfile("strict")
	if err != nil {
		t.Fatalf("strict profile: %v", err)
	}
	cfg.Name = "quality-clone-profile"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Quality = true
	cfg.Checks.Design = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.QualityRules.MaxFunctionLines = 100
	cfg.Checks.QualityRules.MaxParameters = 10
	cfg.Checks.QualityRules.MaxCyclomaticComplexity = 20

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Code Quality", "warn")
	assertFindingRulePresent(t, report, "Code Quality", "quality.duplicate-code")
}
