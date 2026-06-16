package checks_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestQualityCheckWarnsForNativePythonRules(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app.py"), strings.Join([]string{
		"def sample(a, b, c):",
		"    if a:",
		"        return b",
		"    if c:",
		"        return c",
		"    return a",
		"",
	}, "\n"))

	cfg := codeguard.ExampleConfig()
	cfg.Name = "quality-python-native"
	cfg.Targets = []codeguard.TargetConfig{{Name: "api", Path: dir, Language: "python"}}
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
	assertFindingRulePresent(t, report, "Code Quality", "quality.max-function-lines")
	assertFindingRulePresent(t, report, "Code Quality", "quality.max-parameters")
	assertFindingRulePresent(t, report, "Code Quality", "quality.cyclomatic-complexity")
}

func TestQualityCheckFailsForConfiguredTypeScriptCommand(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "index.ts"), "export const answer = 42;\n")
	script := filepath.Join(dir, "fake-tsc.sh")
	writeExecutableFile(t, script, "#!/bin/sh\necho 'src/index.ts:3:1 type error'\nexit 1\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "quality-typescript-command"
	cfg.Targets = []codeguard.TargetConfig{{Name: "web", Path: dir, Language: "typescript"}}
	cfg.Checks.Quality = true
	cfg.Checks.Design = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.QualityRules.LanguageCommands = map[string][]codeguard.CommandCheckConfig{
		"typescript": {{
			Name:    "tsc",
			Command: script,
		}},
	}

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Code Quality", "fail")
	if len(report.Sections[0].Findings) == 0 {
		t.Fatal("expected command finding")
	}
	if !strings.Contains(report.Sections[0].Findings[0].Message, "tsc") {
		t.Fatalf("expected command name in message, got %q", report.Sections[0].Findings[0].Message)
	}
}

func TestQualityCheckWarnsForPythonMaintainability(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app.py"), "def sample(a, b, c):\n    if a:\n        pass\n    if b:\n        pass\n    if c:\n        pass\n    if a and b:\n        pass\n    return a + b + c\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "quality-python-native"
	cfg.Targets = []codeguard.TargetConfig{{Name: "api", Path: dir, Language: "python"}}
	cfg.Checks.Quality = true
	cfg.Checks.Design = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.QualityRules.MaxFunctionLines = 4
	cfg.Checks.QualityRules.MaxParameters = 2
	cfg.Checks.QualityRules.MaxCyclomaticComplexity = 3

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Code Quality", "warn")
}

func TestQualityCheckWarnsForTypeScriptMaintainability(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "sample.ts"), "export function sample(a: number, b: number, c: number) {\n  if (a) {\n    return b;\n  }\n  if (b) {\n    return c;\n  }\n  if (c) {\n    return a;\n  }\n  return a && b ? c : a;\n}\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "quality-typescript-native"
	cfg.Targets = []codeguard.TargetConfig{{Name: "web", Path: dir, Language: "typescript"}}
	cfg.Checks.Quality = true
	cfg.Checks.Design = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.QualityRules.MaxFunctionLines = 5
	cfg.Checks.QualityRules.MaxParameters = 2
	cfg.Checks.QualityRules.MaxCyclomaticComplexity = 3

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Code Quality", "warn")
}
