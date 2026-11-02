package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/cli"
)

// deprecatedConfigFixture writes a valid config that sets every deprecated
// key and returns its path.
func deprecatedConfigFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "codeguard.yaml")
	config := `name: deprecated-keys
targets:
  - name: repo
    path: ` + dir + `
    language: go
checks:
  quality: false
  design: false
  security: false
  prompts: false
  ci: false
  reliability_rules:
    max_retry_attempts: 3
  data_rules:
    max_unbounded_read_rows: 1000
  observability_rules:
    structured_logger_patterns: ["logger."]
  design_rules:
    forbid_internal_import_cmd: true
ai:
  autofix:
    verify_tests: true
rule_packs:
  - name: pack
    rules:
      - id: custom.todo
        title: TODO
        message: todo found
        content_regex: TODO
        ai_prompt: is this a real TODO?
`
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	return configPath
}

var allDeprecatedKeys = []string{
	"checks.reliability_rules.max_retry_attempts",
	"checks.data_rules.max_unbounded_read_rows",
	"checks.observability_rules.structured_logger_patterns",
	"checks.design_rules.forbid_internal_import_cmd",
	"ai.autofix.verify_tests",
	"rule_packs[0].rules[0].ai_prompt",
}

func TestValidateWarnsForEachDeprecatedKeyAndStillPasses(t *testing.T) {
	configPath := deprecatedConfigFixture(t)
	var stdout, stderr bytes.Buffer
	if code := cli.Run([]string{"validate", "-config", configPath}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("validate exit=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "config valid") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	for _, key := range allDeprecatedKeys {
		if !strings.Contains(stderr.String(), "warning: config key "+key+" is deprecated") {
			t.Errorf("validate stderr does not warn about %s:\n%s", key, stderr.String())
		}
	}
}

func TestDoctorWarnsForEachDeprecatedKey(t *testing.T) {
	configPath := deprecatedConfigFixture(t)
	var stdout, stderr bytes.Buffer
	cli.Run([]string{"doctor", "-config", configPath}, strings.NewReader(""), &stdout, &stderr)
	for _, key := range allDeprecatedKeys {
		if !strings.Contains(stdout.String(), "[WARN] config-deprecation: config key "+key+" is deprecated") {
			t.Errorf("doctor output does not warn about %s:\n%s", key, stdout.String())
		}
	}
	if strings.Contains(stdout.String(), "[FAIL] config-deprecation") {
		t.Fatalf("deprecations must warn, not fail:\n%s", stdout.String())
	}
}

func TestValidatePrintsNoDeprecationWarningsForCleanConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "codeguard.yaml")
	if err := os.WriteFile(configPath, []byte("name: clean\ntargets:\n  - name: repo\n    path: "+dir+"\n    language: go\nchecks:\n  quality: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := cli.Run([]string{"validate", "-config", configPath}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("validate exit=%d stderr=%s", code, stderr.String())
	}
	if strings.Contains(stderr.String(), "deprecated") {
		t.Fatalf("unexpected deprecation warning: %s", stderr.String())
	}
}
