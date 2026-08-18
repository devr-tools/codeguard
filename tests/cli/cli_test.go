package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/cli"
	"github.com/devr-tools/codeguard/internal/version"
)

func TestRunVersion(t *testing.T) {
	originalVersion := version.Number
	version.Number = version.Resolve("0.1.0", &debug.BuildInfo{Main: debug.Module{Version: "v1.5.1"}})
	t.Cleanup(func() { version.Number = originalVersion })

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := cli.Run([]string{"version"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if strings.TrimSpace(stdout.String()) == "" {
		t.Fatal("expected version output")
	}
	if got := strings.TrimSpace(stdout.String()); got == "0.1.0" || got != "v1.5.1" {
		t.Fatalf("version output = %q, resolved package version = %q", got, version.Number)
	}
}

func TestRunInitAndValidate(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "codeguard.json")

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if code := cli.Run([]string{"init", "-output", configPath}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("init exit code = %d, stderr = %s", code, stderr.String())
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("expected config file: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if code := cli.Run([]string{"validate", "-config", configPath}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("validate exit code = %d, stderr = %s", code, stderr.String())
	}
}

func TestRunInteractiveInitWritesYAML(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "codeguard.yaml")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	input := strings.NewReader(configPath + "\ninteractive-config\n")
	if code := cli.Run([]string{"init", "-interactive"}, input, &stdout, &stderr); code != 0 {
		t.Fatalf("interactive init exit code = %d, stderr = %s", code, stderr.String())
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("expected yaml config file: %v", err)
	}
}

func TestRunValidateFindsDotCodeguardConfigByDefault(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir tempdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	configDir := filepath.Join(dir, ".codeguard")
	configPath := filepath.Join(configDir, "codeguard.json")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir .codeguard: %v", err)
	}
	config := `{
  "name": "from-dot-codeguard",
  "targets": [{"name": "repo", "path": ".", "language": "go"}],
  "checks": {"quality": false, "design": false, "security": false, "prompts": false, "ci": false},
  "output": {"format": "text"}
}`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := cli.Run([]string{"validate"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("validate exit code = %d, stderr = %s", code, stderr.String())
	}
}

func TestRunValidateAcceptsConfigDirectory(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".codeguard")
	configPath := filepath.Join(configDir, "config.yaml")

	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir .codeguard: %v", err)
	}
	config := `name: from-config-directory
targets:
  - name: repo
    path: .
    language: go
checks:
  quality: false
  design: false
  security: false
  prompts: false
  ci: false
output:
  format: text
`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := cli.Run([]string{"validate", "-config", configDir}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("validate exit code = %d, stderr = %s", code, stderr.String())
	}
}
