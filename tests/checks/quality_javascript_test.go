package checks_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestQualityCheckWarnsForNativeJavaScriptRules(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "index.js"), strings.Join([]string{
		"// @ts-ignore",
		"// @ts-nocheck",
		"// @ts-expect-error",
		"export function sample(input) {",
		"  debugger;",
		"  return input?.value;",
		"}",
		"",
	}, "\n"))

	cfg := codeguard.ExampleConfig()
	cfg.Name = "quality-javascript-native"
	cfg.Targets = []codeguard.TargetConfig{{Name: "web", Path: dir, Language: "javascript"}}
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
	assertFindingRulePresent(t, report, "Code Quality", "quality.javascript.ts-ignore")
	assertFindingRulePresent(t, report, "Code Quality", "quality.javascript.ts-nocheck")
	assertFindingRulePresent(t, report, "Code Quality", "quality.javascript.ts-expect-error")
	assertFindingRulePresent(t, report, "Code Quality", "quality.javascript.debugger-statement")
}

func TestQualityCheckWarnsForNewNativeTypeScriptRules(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "index.ts"), strings.Join([]string{
		"// @ts-expect-error",
		"export function sample(value?: string) {",
		"  debugger;",
		"  return value ?? \"fallback\";",
		"}",
		"",
	}, "\n"))

	cfg := codeguard.ExampleConfig()
	cfg.Name = "quality-typescript-extra"
	cfg.Targets = []codeguard.TargetConfig{{Name: "web", Path: dir, Language: "typescript"}}
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
	assertFindingRulePresent(t, report, "Code Quality", "quality.typescript.ts-expect-error")
	assertFindingRulePresent(t, report, "Code Quality", "quality.typescript.debugger-statement")
}

func TestQualityCheckIgnoresJavaScriptMarkersInsideStrings(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "safe.js"), strings.Join([]string{
		"const examples = [",
		"  \"@ts-expect-error\",",
		"  \"debugger;\",",
		"];",
		"export function sample(input) {",
		"  return examples.concat(input).join(' ');",
		"}",
		"",
	}, "\n"))

	cfg := codeguard.ExampleConfig()
	cfg.Name = "quality-javascript-safe"
	cfg.Targets = []codeguard.TargetConfig{{Name: "web", Path: dir, Language: "javascript"}}
	cfg.Checks.Quality = true
	cfg.Checks.Security = false
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Code Quality", "pass")
}
