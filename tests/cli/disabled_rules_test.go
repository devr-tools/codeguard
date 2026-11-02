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

func disabledRulesConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "codeguard.yaml")
	config := "name: disabled-rules\ntargets:\n  - name: repo\n    path: " + dir + "\n    language: go\nchecks:\n  quality: true\n  observability: true\n  disabled_rules:\n    - observability.unstructured-log\n"
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	return configPath
}

func TestExplainReportsRuleDisabledByDisabledRules(t *testing.T) {
	configPath := disabledRulesConfig(t)
	var stdout, stderr bytes.Buffer
	if code := cli.Run([]string{"explain", "-config", configPath, "observability.unstructured-log"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("explain exit=%d stderr=%s", code, stderr.String())
	}
	text := stdout.String()
	if !strings.Contains(text, "enabled under this config: no") || !strings.Contains(text, "disabled by checks.disabled_rules: observability.unstructured-log") {
		t.Fatalf("explain text does not report the disabled rule:\n%s", text)
	}

	stdout.Reset()
	stderr.Reset()
	if code := cli.Run([]string{"explain", "-config", configPath, "-format", "json", "observability.unstructured-log"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("explain json exit=%d stderr=%s", code, stderr.String())
	}
	var out struct {
		Enabled    *bool  `json:"enabled"`
		DisabledBy string `json:"disabled_by"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Enabled == nil || *out.Enabled || out.DisabledBy != "checks.disabled_rules: observability.unstructured-log" {
		t.Fatalf("explain json = %+v", out)
	}
}

func TestRulesEnabledListsDisabledRulesSeparately(t *testing.T) {
	configPath := disabledRulesConfig(t)
	var stdout, stderr bytes.Buffer
	if code := cli.Run([]string{"rules", "-config", configPath, "-enabled", "-section", "observability"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("rules exit=%d stderr=%s", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "observability.unstructured-log\t") {
		t.Fatalf("disabled rule listed as enabled:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "observability.") {
		t.Fatalf("other observability rules should still be listed:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "disabled: observability.unstructured-log (checks.disabled_rules: observability.unstructured-log)") {
		t.Fatalf("stderr does not list the disabled rule:\n%s", stderr.String())
	}
}

func TestValidateRejectsUnknownDisabledRuleWithSuggestion(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "codeguard.yaml")
	config := "name: typo\ntargets:\n  - name: repo\n    path: " + dir + "\n    language: go\nchecks:\n  quality: true\n  disabled_rules: [observability.unstructured-logs]\n"
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := cli.Run([]string{"validate", "-config", configPath}, strings.NewReader(""), &stdout, &stderr); code == 0 {
		t.Fatal("validate should reject an unknown disabled rule")
	}
	if !strings.Contains(stderr.String(), `did you mean "observability.unstructured-log"`) {
		t.Fatalf("stderr = %s", stderr.String())
	}
}
