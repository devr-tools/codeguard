package checks_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestQualityCheckFailsForUnformattedGoFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), "package main\nfunc main(){println(\"hi\")}\n")

	report, err := codeguard.Run(context.Background(), codeguard.Config{
		Name: "quality-test",
		Targets: []codeguard.TargetConfig{{
			Name:     "repo",
			Path:     dir,
			Language: "go",
		}},
		Checks: codeguard.CheckConfig{
			Quality: true,
		},
		Output: codeguard.OutputConfig{Format: "text"},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Code Quality", "fail")
}

func TestQualityCheckWarnsForMaintainabilityThresholds(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc sample(a, b int) int {\n\treturn a + b\n}\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "quality-threshold-test"
	cfg.Targets = []codeguard.TargetConfig{{
		Name:     "repo",
		Path:     dir,
		Language: "go",
	}}
	cfg.Checks.Quality = true
	cfg.Checks.QualityRules.MaxFunctionLines = 1
	cfg.Checks.QualityRules.MaxParameters = 1
	cfg.Checks.Security = false
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Code Quality", "warn")
}

func TestQualityCheckWarnsForCyclomaticComplexity(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc sample(a int) int {\n\tif a > 0 {\n\t\ta++\n\t}\n\tif a > 1 {\n\t\ta++\n\t}\n\tif a > 2 {\n\t\ta++\n\t}\n\treturn a\n}\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "quality-complexity-test"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Quality = true
	cfg.Checks.QualityRules.MaxCyclomaticComplexity = 2
	cfg.Checks.Security = false
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Code Quality", "warn")
}

func TestQualityCheckWarnsForOversizedFileWithoutComplexity(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), strings.Join([]string{
		"package main",
		"",
		"func first() int {",
		"\treturn 1",
		"}",
		"",
		"func second() int {",
		"\treturn 2",
		"}",
		"",
	}, "\n"))

	cfg := codeguard.ExampleConfig()
	cfg.Name = "quality-file-length-warn"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Quality = true
	cfg.Checks.QualityRules.MaxFileLines = 5
	cfg.Checks.QualityRules.MaxCyclomaticComplexity = 10
	cfg.Checks.Security = false
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Code Quality", "warn")
	assertFindingRulePresent(t, report, "Code Quality", "quality.max-file-lines")
	assertFindingLevel(t, report, "Code Quality", "quality.max-file-lines", "warn")
	assertFindingRuleAbsent(t, report, "Code Quality", "quality.cyclomatic-complexity")
}

func TestQualityCheckFailsForOversizedFileWithComplexity(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), strings.Join([]string{
		"package main",
		"",
		"func sample(a int) int {",
		"\tif a > 0 {",
		"\t\ta++",
		"\t}",
		"\tif a > 1 {",
		"\t\ta++",
		"\t}",
		"\tif a > 2 {",
		"\t\ta++",
		"\t}",
		"\treturn a",
		"}",
		"",
	}, "\n"))

	cfg := codeguard.ExampleConfig()
	cfg.Name = "quality-file-length-fail"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Quality = true
	cfg.Checks.QualityRules.MaxFileLines = 5
	cfg.Checks.QualityRules.MaxCyclomaticComplexity = 2
	cfg.Checks.Security = false
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Code Quality", "fail")
	assertFindingRulePresent(t, report, "Code Quality", "quality.max-file-lines")
	assertFindingRulePresent(t, report, "Code Quality", "quality.cyclomatic-complexity")
	assertFindingLevel(t, report, "Code Quality", "quality.max-file-lines", "fail")
	assertFindingLevel(t, report, "Code Quality", "quality.cyclomatic-complexity", "warn")
}

func TestQualityCheckWarnsForDependencyDirection(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "lib.go"), "package sample\n\nimport cli \"github.com/devr-tools/codeguard/internal/cli\"\n\nvar _ = cli.Run\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "quality-deps-test"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Quality = true
	cfg.Checks.Security = false
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Code Quality", "warn")
}

func TestQualityCheckWarnsForNativeTypeScriptRules(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "index.ts"), strings.Join([]string{
		"// @ts-ignore",
		"// @ts-nocheck",
		"export function sample(input?: { value?: string }, a: any, b: string, c: number) {",
		"  const forced = input!.value as unknown as string;",
		"  if (a || forced) {",
		"    return b;",
		"  }",
		"  if (c > 0) {",
		"    return `${c}` as any;",
		"  }",
		"  return b;",
		"}",
		"",
	}, "\n"))

	cfg := codeguard.ExampleConfig()
	cfg.Name = "quality-typescript-native"
	cfg.Targets = []codeguard.TargetConfig{{Name: "web", Path: dir, Language: "typescript"}}
	cfg.Checks.Quality = true
	cfg.Checks.Security = false
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.QualityRules.MaxFunctionLines = 4
	cfg.Checks.QualityRules.MaxParameters = 2
	cfg.Checks.QualityRules.MaxCyclomaticComplexity = 2

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Code Quality", "warn")
	assertFindingRulePresent(t, report, "Code Quality", "quality.typescript.ts-ignore")
	assertFindingRulePresent(t, report, "Code Quality", "quality.typescript.ts-nocheck")
	assertFindingRulePresent(t, report, "Code Quality", "quality.typescript.explicit-any")
	assertFindingRulePresent(t, report, "Code Quality", "quality.typescript.double-assertion")
	assertFindingRulePresent(t, report, "Code Quality", "quality.typescript.non-null-assertion")
	assertFindingRulePresent(t, report, "Code Quality", "quality.max-function-lines")
	assertFindingRulePresent(t, report, "Code Quality", "quality.max-parameters")
	assertFindingRulePresent(t, report, "Code Quality", "quality.cyclomatic-complexity")
}

func TestQualityCheckIgnoresTypeScriptPatternsInStringsAndComments(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "safe.ts"), strings.Join([]string{
		"const examples = [",
		"  \"value as any\",",
		"  \"input!.value\",",
		"  \"node.innerHTML = value\",",
		"  \"@ts-nocheck\",",
		"];",
		"// example markers only",
		"export function sample(input: string) {",
		"  return input.trim();",
		"}",
		"",
	}, "\n"))

	cfg := codeguard.ExampleConfig()
	cfg.Name = "quality-typescript-safe"
	cfg.Targets = []codeguard.TargetConfig{{Name: "web", Path: dir, Language: "typescript"}}
	cfg.Checks.Quality = true
	cfg.Checks.Security = false
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.QualityRules.MaxFunctionLines = 20
	cfg.Checks.QualityRules.MaxParameters = 5
	cfg.Checks.QualityRules.MaxCyclomaticComplexity = 5

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Code Quality", "pass")
}
