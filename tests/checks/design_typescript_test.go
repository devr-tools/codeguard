package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestDesignCheckWarnsForGenericJavaScriptModuleName(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "util.js"), "export const answer = 42;\n")

	report, err := codeguard.Run(context.Background(), typeScriptDesignConfig(dir, "javascript"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Design Patterns", "warn")
	assertFindingRulePresent(t, report, "Design Patterns", "design.typescript.generic-module-name")
}

func TestDesignCheckWarnsForTypeScriptClassWithTooManyMethods(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "service.ts"), "export class Service {\n  one() {}\n  two() {}\n  three() {}\n}\n")

	cfg := typeScriptDesignConfig(dir, "typescript")
	cfg.Checks.DesignRules.MaxMethodsPerType = 2

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Design Patterns", "warn")
	assertFindingRulePresent(t, report, "Design Patterns", "design.typescript.max-methods-per-type")
}

func TestDesignCheckWarnsForLargeTypeScriptInterface(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "ports.ts"), "export interface Client {\n  one(): void;\n  two(): void;\n  three(): void;\n}\n")

	cfg := typeScriptDesignConfig(dir, "typescript")
	cfg.Checks.DesignRules.MaxInterfaceMethods = 2

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Design Patterns", "warn")
	assertFindingRulePresent(t, report, "Design Patterns", "design.typescript.max-interface-members")
}

func TestDesignCheckPassesForWellFactoredTypeScriptLayout(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "service.ts"), "export class Service {\n  one() {}\n  two() {}\n}\n")
	writeFile(t, filepath.Join(dir, "src", "ports.ts"), "export type Client = {\n  one(): void;\n  two(): void;\n};\n")

	cfg := typeScriptDesignConfig(dir, "typescript")
	cfg.Checks.DesignRules.MaxMethodsPerType = 2
	cfg.Checks.DesignRules.MaxInterfaceMethods = 2

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Design Patterns", "pass")
}

func typeScriptDesignConfig(dir string, language string) codeguard.Config {
	cfg := codeguard.ExampleConfig()
	cfg.Name = "design-typescript"
	cfg.Targets = []codeguard.TargetConfig{{Name: "web", Path: dir, Language: language}}
	cfg.Checks.Design = true
	cfg.Checks.Quality = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	return cfg
}
