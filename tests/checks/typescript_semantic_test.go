package checks_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func TestDesignCheckSkipsLargeTypeScriptDataContractsAndProps(t *testing.T) {
	requireTypeScriptSemanticRuntime(t)

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "types", "tool.tsx"), strings.Join([]string{
		"export interface ToolDefinition {",
		"  id: string;",
		"  name: string;",
		"  description: string;",
		"  version: string;",
		"  owner: string;",
		"  fields: ToolField[];",
		"  enabled: boolean;",
		"}",
		"export interface ToolField {",
		"  key: string;",
		"  label: string;",
		"  type: string;",
		"  required: boolean;",
		"  options: string[];",
		"  defaultValue: string;",
		"}",
		"export type LmpAbuseConfig = {",
		"  threshold: number;",
		"  windowSeconds: number;",
		"  highVolume: boolean;",
		"  customAvatar: boolean;",
		"  alertChannel: string;",
		"  owner: string;",
		"};",
		"export interface ToolPanelProps {",
		"  tool: ToolDefinition;",
		"  fields: ToolField[];",
		"  selected: string[];",
		"  loading: boolean;",
		"  onSave(): void;",
		"  onCancel(): void;",
		"}",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "src", "domain", "wide-policy.ts"), strings.Join([]string{
		"export interface WidePolicy {",
		"  one(): void;",
		"  two(): void;",
		"  three(): void;",
		"  four(): void;",
		"  five(): void;",
		"  six(): void;",
		"}",
	}, "\n"))

	cfg := typeScriptDesignConfig(dir, "typescript")
	cfg.Checks.DesignRules.MaxInterfaceMethods = 5

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Design Patterns", "design.typescript.max-interface-members")
	for _, section := range report.Sections {
		if section.Name != "Design Patterns" {
			continue
		}
		for _, finding := range section.Findings {
			if finding.RuleID != "design.typescript.max-interface-members" {
				continue
			}
			if strings.Contains(finding.Message, "ToolDefinition") ||
				strings.Contains(finding.Message, "ToolField") ||
				strings.Contains(finding.Message, "LmpAbuseConfig") ||
				strings.Contains(finding.Message, "ToolPanelProps") {
				t.Fatalf("data contracts and prop interfaces should not trip max-interface-members: %+v", finding)
			}
		}
	}
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

func TestSecuritySemanticAnalyzerScansNodeModulesWithinTarget(t *testing.T) {
	requireTypeScriptSemanticRuntime(t)

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "node_modules", "local-package", "index.ts"), "const agent = { rejectUnauthorized: false };\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "security-typescript-semantic-node-modules"
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

	assertSectionStatus(t, report, "Security", "fail")
	assertFindingRulePresent(t, report, "Security", "security.typescript.insecure-tls")
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
