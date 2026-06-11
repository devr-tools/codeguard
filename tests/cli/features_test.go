package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/cli"
	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestRunRules(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := cli.Run([]string{"rules"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "quality.gofmt") {
		t.Fatalf("expected rules output, got: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "quality.gofmt\tfail\tgo-native\tgo\tCode Quality\tGo formatting") {
		t.Fatalf("expected execution model and language coverage in rules output, got: %s", stdout.String())
	}
}

func TestRunExplain(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := cli.Run([]string{"explain", "security.hardcoded-secret"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d, stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Hardcoded secret") && !strings.Contains(stdout.String(), "hardcoded secret") {
		t.Fatalf("expected explain output, got: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "execution model: language-agnostic") {
		t.Fatalf("expected execution model in explain output, got: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "language coverage: repository-wide") {
		t.Fatalf("expected language coverage in explain output, got: %s", stdout.String())
	}
}

func TestRunBaselineWritesFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "codeguard.json")
	baselinePath := filepath.Join(dir, "codeguard-baseline.json")
	promptPath := filepath.Join(dir, "prompts", "system.prompt")
	if err := os.MkdirAll(filepath.Dir(promptPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(promptPath, []byte("Use ${OPENAI_API_KEY} for downstream calls.\n"), 0o644); err != nil {
		t.Fatalf("write prompt: %v", err)
	}

	config := `{
  "name": "baseline-cli-test",
  "targets": [{"name": "repo", "path": "` + dir + `", "language": "go"}],
  "checks": {"quality": false, "design": false, "security": false, "prompts": true, "ci": false},
  "output": {"format": "text"}
}`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := cli.Run([]string{"baseline", "-config", configPath, "-output", baselinePath}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d, stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(baselinePath); err != nil {
		t.Fatalf("expected baseline file: %v", err)
	}
}

func TestRunRulesWithConfigIncludesCustomRules(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "codeguard.json")
	config := `{
  "name": "custom-rule-cli",
  "targets": [{"name": "repo", "path": "` + dir + `", "language": "go"}],
  "checks": {"quality": false, "design": false, "security": false, "prompts": false, "ci": false},
  "output": {"format": "text"},
  "rule_packs": [{
    "name": "repo-policy",
    "rules": [{
      "id": "custom.disallow-env",
      "title": "Disallow env files",
      "severity": "fail",
      "message": "env files must not be committed",
      "paths": [".env"],
      "file_extensions": [".env"]
    }]
  }]
}`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := cli.Run([]string{"rules", "-config", configPath}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d, stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "custom.disallow-env") {
		t.Fatalf("expected custom rule in rules output, got: %s", stdout.String())
	}
}

func TestSDKRuleMetadataIncludesExecutionModel(t *testing.T) {
	rule, ok := codeguard.ExplainRule("quality.gofmt")
	if !ok {
		t.Fatal("expected quality.gofmt metadata")
	}
	if rule.ExecutionModel != codeguard.RuleExecutionModelGoNative {
		t.Fatalf("quality.gofmt execution model = %q, want %q", rule.ExecutionModel, codeguard.RuleExecutionModelGoNative)
	}
	if rule.LanguageCoverage.Mode != codeguard.RuleLanguageCoverageFixed {
		t.Fatalf("quality.gofmt language coverage mode = %q, want %q", rule.LanguageCoverage.Mode, codeguard.RuleLanguageCoverageFixed)
	}
	if !reflect.DeepEqual(rule.LanguageCoverage.Languages, []codeguard.RuleLanguage{codeguard.RuleLanguageGo}) {
		t.Fatalf("quality.gofmt language coverage languages = %#v, want %#v", rule.LanguageCoverage.Languages, []codeguard.RuleLanguage{codeguard.RuleLanguageGo})
	}

	rule, ok = codeguard.ExplainRule("quality.max-function-lines")
	if !ok {
		t.Fatal("expected quality.max-function-lines metadata")
	}
	if rule.ExecutionModel != codeguard.RuleExecutionModelLanguageAgnostic {
		t.Fatalf("quality.max-function-lines execution model = %q, want %q", rule.ExecutionModel, codeguard.RuleExecutionModelLanguageAgnostic)
	}
	if rule.LanguageCoverage.Mode != codeguard.RuleLanguageCoverageFixed {
		t.Fatalf("quality.max-function-lines language coverage mode = %q, want %q", rule.LanguageCoverage.Mode, codeguard.RuleLanguageCoverageFixed)
	}
	if !reflect.DeepEqual(rule.LanguageCoverage.Languages, []codeguard.RuleLanguage{codeguard.RuleLanguageGo, codeguard.RuleLanguagePython, codeguard.RuleLanguageTypeScript}) {
		t.Fatalf("quality.max-function-lines language coverage languages = %#v, want %#v", rule.LanguageCoverage.Languages, []codeguard.RuleLanguage{codeguard.RuleLanguageGo, codeguard.RuleLanguagePython, codeguard.RuleLanguageTypeScript})
	}

	rule, ok = codeguard.ExplainRule("quality.typescript.explicit-any")
	if !ok {
		t.Fatal("expected quality.typescript.explicit-any metadata")
	}
	if rule.LanguageCoverage.Mode != codeguard.RuleLanguageCoverageFixed {
		t.Fatalf("quality.typescript.explicit-any language coverage mode = %q, want %q", rule.LanguageCoverage.Mode, codeguard.RuleLanguageCoverageFixed)
	}
	if !reflect.DeepEqual(rule.LanguageCoverage.Languages, []codeguard.RuleLanguage{codeguard.RuleLanguageTypeScript}) {
		t.Fatalf("quality.typescript.explicit-any language coverage languages = %#v, want %#v", rule.LanguageCoverage.Languages, []codeguard.RuleLanguage{codeguard.RuleLanguageTypeScript})
	}

	rule, ok = codeguard.ExplainRule("security.command-check")
	if !ok {
		t.Fatal("expected security.command-check metadata")
	}
	if rule.ExecutionModel != codeguard.RuleExecutionModelCommandDriven {
		t.Fatalf("security.command-check execution model = %q, want %q", rule.ExecutionModel, codeguard.RuleExecutionModelCommandDriven)
	}
	if rule.LanguageCoverage.Mode != codeguard.RuleLanguageCoverageConfigurable {
		t.Fatalf("security.command-check language coverage mode = %q, want %q", rule.LanguageCoverage.Mode, codeguard.RuleLanguageCoverageConfigurable)
	}

	rule, ok = codeguard.ExplainRule("security.hardcoded-secret")
	if !ok {
		t.Fatal("expected security.hardcoded-secret metadata")
	}
	if rule.LanguageCoverage.Mode != codeguard.RuleLanguageCoverageRepositoryWide {
		t.Fatalf("security.hardcoded-secret language coverage mode = %q, want %q", rule.LanguageCoverage.Mode, codeguard.RuleLanguageCoverageRepositoryWide)
	}

	cfg := codeguard.ExampleConfig()
	cfg.RulePacks = []codeguard.RulePackConfig{{
		Name: "repo-policy",
		Rules: []codeguard.CustomRuleConfig{{
			ID:       "custom.disallow-env",
			Title:    "Disallow env files",
			Severity: "fail",
			Message:  "env files must not be committed",
			Paths:    []string{".env"},
		}},
	}}

	var customRule codeguard.RuleMetadata
	for _, meta := range codeguard.RulesForConfig(cfg) {
		if meta.ID == "custom.disallow-env" {
			customRule = meta
			break
		}
	}
	if customRule.ID == "" {
		t.Fatal("expected custom.disallow-env metadata")
	}
	if customRule.ExecutionModel != codeguard.RuleExecutionModelLanguageAgnostic {
		t.Fatalf("custom.disallow-env execution model = %q, want %q", customRule.ExecutionModel, codeguard.RuleExecutionModelLanguageAgnostic)
	}
	if customRule.LanguageCoverage.Mode != codeguard.RuleLanguageCoverageConfigurable {
		t.Fatalf("custom.disallow-env language coverage mode = %q, want %q", customRule.LanguageCoverage.Mode, codeguard.RuleLanguageCoverageConfigurable)
	}
}

func TestRunDoctor(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "codeguard.json")
	config := `{
  "name": "doctor-cli-test",
  "targets": [{"name": "repo", "path": "` + dir + `", "language": "go"}],
  "checks": {"quality": false, "design": false, "security": false, "prompts": false, "ci": false},
  "output": {"format": "text"}
}`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := cli.Run([]string{"doctor", "-config", configPath}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d, stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "[PASS] config:") {
		t.Fatalf("expected doctor output, got: %s", stdout.String())
	}
}

func TestRunProfiles(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := cli.Run([]string{"profiles"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "strict") {
		t.Fatalf("expected strict profile output, got: %s", stdout.String())
	}
}
