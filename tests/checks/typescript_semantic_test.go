package checks_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestDesignCheckUsesSemanticTypeScriptAnalyzerForAnonymousDefaultClass(t *testing.T) {
	requireTypeScriptSemanticRuntime(t)

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "service.ts"), "export default class {\n  one() {}\n  two() {}\n  three() {}\n}\n")

	cfg := typeScriptDesignConfig(dir, "typescript")
	cfg.Checks.DesignRules.MaxMethodsPerType = 2

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Design Patterns", "warn")
	assertFindingRulePresent(t, report, "Design Patterns", "design.typescript.max-methods-per-type")
}

func TestQualityCheckUsesSemanticTypeScriptAnalyzerForClassArrowMethods(t *testing.T) {
	requireTypeScriptSemanticRuntime(t)

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "service.ts"), "export class Service {\n  run = (a: number, b: number, c: number) => {\n    if (a) return b;\n    if (b) return c;\n    if (c) return a;\n    return a && b ? c : a;\n  };\n}\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "quality-typescript-semantic"
	cfg.Targets = []codeguard.TargetConfig{{Name: "web", Path: dir, Language: "typescript"}}
	cfg.Checks.Quality = true
	cfg.Checks.Design = false
	cfg.Checks.Security = false
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

func TestSecurityCheckUsesSemanticTypeScriptAnalyzerForRequirePropertyAlias(t *testing.T) {
	requireTypeScriptSemanticRuntime(t)

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "index.ts"), "const exec = require(\"node:child_process\").exec;\nexec(\"echo hi\");\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "security-typescript-semantic"
	cfg.Targets = []codeguard.TargetConfig{{Name: "web", Path: dir, Language: "typescript"}}
	cfg.Checks.Security = true
	cfg.Checks.Design = false
	cfg.Checks.Quality = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Security", "warn")
	assertFindingRulePresent(t, report, "Security", "security.typescript.shell-execution")
}

func requireTypeScriptSemanticRuntime(t *testing.T) {
	t.Helper()

	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node runtime not available")
	}

	for _, candidate := range semanticTypeScriptLibCandidates() {
		if _, err := os.Stat(candidate); err == nil {
			t.Setenv("CODEGUARD_TYPESCRIPT_LIB_PATH", candidate)
			return
		}
	}

	t.Skip("TypeScript semantic runtime not available")
}

func semanticTypeScriptLibCandidates() []string {
	candidates := make([]string, 0, 8)
	if value := os.Getenv("CODEGUARD_TYPESCRIPT_LIB_PATH"); value != "" {
		candidates = append(candidates, value)
	}
	if cwd, err := os.Getwd(); err == nil {
		for _, dir := range ancestorPaths(cwd) {
			candidates = append(candidates, filepath.Join(dir, "node_modules", "typescript", "lib", "typescript.js"))
		}
	}
	candidates = append(candidates, "/Applications/Visual Studio Code.app/Contents/Resources/app/extensions/node_modules/typescript/lib/typescript.js")
	return candidates
}

func ancestorPaths(path string) []string {
	paths := make([]string, 0, 6)
	current := path
	for {
		paths = append(paths, current)
		parent := filepath.Dir(current)
		if parent == current {
			return paths
		}
		current = parent
	}
}
