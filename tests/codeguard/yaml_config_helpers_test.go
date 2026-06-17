package codeguard_test

import (
	"os"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func yamlRoundTripConfig() codeguard.Config {
	cfg := codeguard.ExampleConfig()
	cfg.Checks.SupplyChain = true
	cfg.Checks.QualityRules.LanguageCommands = map[string][]codeguard.CommandCheckConfig{
		"typescript": {{Name: "tsc", Command: "npx", Args: []string{"tsc", "--noEmit"}}},
	}
	cfg.Checks.DesignRules.LanguageCommands = map[string][]codeguard.CommandCheckConfig{
		"python": {{Name: "import-linter", Command: "lint-imports", Args: []string{"--config", "importlinter.ini"}}},
	}
	cfg.Checks.DesignRules.LanguageDiffCommands = map[string][]codeguard.CommandCheckConfig{
		"go": {{Name: "api-diff", Command: "./scripts/api-diff.sh"}},
	}
	cfg.AI.AutoFix.TestCommands = []codeguard.CommandCheckConfig{{
		Name: "unit", Command: "go", Args: []string{"test", "./..."},
	}}
	cfg.RulePacks = []codeguard.RulePackConfig{{
		Name: "repo-policy", Description: "Example pack", Rules: []codeguard.CustomRuleConfig{{
			ID: "custom.no-debug", Title: "No debug", Message: "Debug code is forbidden", HowToFix: "Remove debug code", Paths: []string{"**/*.go"},
		}},
	}}
	return cfg
}

func assertYAMLSchemaMarkers(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read yaml: %v", err)
	}
	rendered := string(data)
	for _, want := range []string{"supply_chain:", "quality_rules:", "max_file_lines:", "language_commands:", "ci_rules:", "required_workflow_files:", "hybrid_triage:", "candidate_sections:", "function_contract:", "test_commands:", "rule_packs:"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("written yaml missing %q:\n%s", want, rendered)
		}
	}
}

func assertYAMLRoundTripConfig(t *testing.T, loaded codeguard.Config, want codeguard.Config) {
	t.Helper()
	if loaded.Name != want.Name {
		t.Fatalf("loaded name = %q, want %q", loaded.Name, want.Name)
	}
	assertYAMLCommand(t, loaded.Checks.QualityRules.LanguageCommands["typescript"][0].Command, "npx", "loaded command")
	assertYAMLCommand(t, loaded.Checks.DesignRules.LanguageCommands["python"][0].Command, "lint-imports", "loaded design command")
	assertYAMLCommand(t, loaded.Checks.DesignRules.LanguageDiffCommands["go"][0].Name, "api-diff", "loaded diff command")
}

func assertYAMLCommand(t *testing.T, got string, want string, label string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %q, want %q", label, got, want)
	}
}

func snakeCaseYAMLFixture() string {
	return `name: snake-case-config
targets:
  - name: repo
    path: .
    language: go
checks:
  quality: false
  design: false
  security: false
  prompts: false
  ci: true
  supply_chain: true
  quality_rules:
    max_file_lines: 123
    coverage_delta:
      enabled: true
      min_changed_line_coverage: 77
  ci_rules:
    require_workflow_dir: true
    required_workflow_files:
      - .github/workflows/ci.yml
  supply_chain_rules:
    require_lockfile: true
    detect_lockfile_drift: true
    detect_unpinned: true
ai:
  enabled: true
  hybrid_triage:
    enabled: true
    candidate_sections:
      - Code Quality
  semantic:
    enabled: true
    function_contract: true
    test_adequacy: true
  autofix:
    enabled: true
    verify_tests: true
    max_fixes: 2
    test_commands:
      - name: unit
        command: go
        args: ["test", "./..."]
rule_packs:
  - name: repo-policy
    description: Example pack
    rules:
      - id: custom.no-debug
        title: No debug
        message: Debug code is forbidden
        how_to_fix: Remove debug code
        natural_language: reject debug code
        paths:
          - "**/*.go"
output:
  format: text
`
}

func assertSnakeCaseYAMLLoaded(t *testing.T, loaded codeguard.Config) {
	t.Helper()
	assertSnakeCaseChecks(t, loaded)
	assertSnakeCaseAI(t, loaded)
	assertSnakeCaseRulePack(t, loaded)
}

func assertSnakeCaseChecks(t *testing.T, loaded codeguard.Config) {
	t.Helper()
	if !loaded.Checks.SupplyChain {
		t.Fatal("expected supply_chain to load from snake_case yaml")
	}
	if loaded.Checks.QualityRules.MaxFileLines != 123 {
		t.Fatalf("max_file_lines = %d, want 123", loaded.Checks.QualityRules.MaxFileLines)
	}
	if loaded.Checks.CIRules.RequireWorkflowDir == nil || !*loaded.Checks.CIRules.RequireWorkflowDir {
		t.Fatal("expected require_workflow_dir to load")
	}
	if loaded.Checks.QualityRules.CoverageDelta.Enabled == nil || !*loaded.Checks.QualityRules.CoverageDelta.Enabled {
		t.Fatal("expected coverage_delta.enabled to load")
	}
	if loaded.Checks.QualityRules.CoverageDelta.MinChangedLineCoverage == nil || *loaded.Checks.QualityRules.CoverageDelta.MinChangedLineCoverage != 77 {
		t.Fatalf("min_changed_line_coverage = %#v, want 77", loaded.Checks.QualityRules.CoverageDelta.MinChangedLineCoverage)
	}
}

func assertSnakeCaseAI(t *testing.T, loaded codeguard.Config) {
	t.Helper()
	if loaded.AI.HybridTriage.Enabled == nil || !*loaded.AI.HybridTriage.Enabled {
		t.Fatal("expected hybrid_triage.enabled to load")
	}
	if len(loaded.AI.HybridTriage.CandidateSections) != 1 || loaded.AI.HybridTriage.CandidateSections[0] != "Code Quality" {
		t.Fatalf("candidate_sections = %#v, want [Code Quality]", loaded.AI.HybridTriage.CandidateSections)
	}
	if loaded.AI.Semantic.FunctionContract == nil || !*loaded.AI.Semantic.FunctionContract {
		t.Fatal("expected semantic.function_contract to load")
	}
	if loaded.AI.Semantic.TestAdequacy == nil || !*loaded.AI.Semantic.TestAdequacy {
		t.Fatal("expected semantic.test_adequacy to load")
	}
	if loaded.AI.AutoFix.MaxFixes != 2 {
		t.Fatalf("max_fixes = %d, want 2", loaded.AI.AutoFix.MaxFixes)
	}
	if len(loaded.AI.AutoFix.TestCommands) != 1 || loaded.AI.AutoFix.TestCommands[0].Name != "unit" {
		t.Fatalf("test_commands = %#v, want one named unit", loaded.AI.AutoFix.TestCommands)
	}
}

func assertSnakeCaseRulePack(t *testing.T, loaded codeguard.Config) {
	t.Helper()
	if len(loaded.RulePacks) != 1 || loaded.RulePacks[0].Rules[0].HowToFix != "Remove debug code" {
		t.Fatalf("rule_packs = %#v, want custom rule with how_to_fix", loaded.RulePacks)
	}
}
