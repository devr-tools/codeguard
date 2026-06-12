package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/cli"
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

func TestRunExplainAgentFormat(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := cli.Run([]string{"explain", "-format", "agent", "security.hardcoded-secret"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d, stderr=%s", code, stderr.String())
	}

	var payload struct {
		ID               string `json:"id"`
		Title            string `json:"title"`
		Section          string `json:"section"`
		Level            string `json:"level"`
		ExecutionModel   string `json:"execution_model"`
		Description      string `json:"description"`
		Why              string `json:"why"`
		HowToFix         string `json:"how_to_fix"`
		FixTemplate      string `json:"fix_template"`
		LanguageCoverage struct {
			Mode      string   `json:"mode"`
			Languages []string `json:"languages"`
		} `json:"language_coverage"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json, got err=%v body=%s", err, stdout.String())
	}

	if payload.ID != "security.hardcoded-secret" {
		t.Fatalf("expected rule id, got %#v", payload)
	}
	if payload.ExecutionModel != "language-agnostic" {
		t.Fatalf("expected execution model, got %#v", payload)
	}
	if payload.LanguageCoverage.Mode != "repository-wide" {
		t.Fatalf("expected repository-wide coverage, got %#v", payload.LanguageCoverage)
	}
	if len(payload.LanguageCoverage.Languages) != 0 {
		t.Fatalf("expected empty languages for repository-wide coverage, got %#v", payload.LanguageCoverage.Languages)
	}
	if payload.Description == "" || payload.Why == "" {
		t.Fatalf("expected description and why, got %#v", payload)
	}
	if payload.HowToFix == "" {
		t.Fatalf("expected how_to_fix, got %#v", payload)
	}
	if payload.FixTemplate != "" {
		t.Fatalf("expected empty fix_template without explicit metadata, got %#v", payload)
	}
}

func TestRunValidatePatchUsesPatchedContent(t *testing.T) {
	configPath, promptPath := writePromptPolicyFixture(t, "patch-cli-test", "json", "Keep prompts generic.\n")
	diff := promptSecretPatchDiff()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := cli.Run([]string{"validate-patch", "-config", configPath, "-format", "json"}, strings.NewReader(diff), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1 for failing patch, got %d, stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}

	report := decodeValidatePatchReport(t, stdout.Bytes(), stdout.String())
	assertPatchedContentFinding(t, report)
	assertPromptFileUnchanged(t, promptPath)
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

func TestRunRulesWithConfigIncludesNaturalLanguageExecutionModel(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "codeguard.json")
	config := `{
  "name": "custom-rule-cli-nl",
  "targets": [{"name": "repo", "path": "` + dir + `", "language": "go"}],
  "checks": {"quality": false, "design": false, "security": false, "prompts": false, "ci": false},
  "output": {"format": "text"},
  "rule_packs": [{
    "name": "repo-policy",
    "rules": [{
      "id": "custom.no-request-body-logs",
      "title": "Never log request bodies",
      "severity": "fail",
      "message": "request bodies must not be logged in handlers",
      "natural_language": "never log request bodies in handlers",
      "paths": ["handlers/**"]
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
	if !strings.Contains(stdout.String(), "custom.no-request-body-logs\tfail\tcommand-driven\tconfigurable\tCustom Rules\tNever log request bodies") {
		t.Fatalf("expected command-driven natural-language rule metadata, got: %s", stdout.String())
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
