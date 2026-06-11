package cli_test

import (
	"bytes"
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
