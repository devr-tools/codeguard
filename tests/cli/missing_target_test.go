package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/cli"
)

func writeMissingTargetConfig(t *testing.T, dir string, targetPath string) string {
	t.Helper()
	configPath := filepath.Join(dir, "codeguard.yaml")
	config := "name: missing-target\n" +
		"targets:\n" +
		"  - name: app\n" +
		"    path: " + targetPath + "\n" +
		"    language: go\n" +
		"checks:\n" +
		"  quality: true\n" +
		"  design: false\n" +
		"  security: false\n" +
		"  prompts: false\n" +
		"  ci: false\n" +
		"  performance: false\n" +
		"  context: false\n"
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return configPath
}

func TestRunValidateRejectsNonexistentTargetPath(t *testing.T) {
	configPath := writeMissingTargetConfig(t, t.TempDir(), "./does-not-exist")

	var stdout, stderr bytes.Buffer
	code := cli.Run([]string{"validate", "-config", configPath}, strings.NewReader(""), &stdout, &stderr)
	if code == 0 {
		t.Fatalf("validate exit code = 0 for a nonexistent target; stdout = %s", stdout.String())
	}
	if strings.Contains(stdout.String(), "config valid") {
		t.Fatalf("validate printed %q for a nonexistent target", stdout.String())
	}
	if !strings.Contains(stderr.String(), "does-not-exist") || !strings.Contains(stderr.String(), "does not exist") {
		t.Fatalf("stderr = %q, want it to name the missing target path", stderr.String())
	}
}

func TestRunValidateRejectsNonDirectoryTargetPath(t *testing.T) {
	dir := t.TempDir()
	writeScanTestFile(t, filepath.Join(dir, "main.go"), "package main\n")
	configPath := writeMissingTargetConfig(t, dir, "./main.go")

	var stdout, stderr bytes.Buffer
	code := cli.Run([]string{"validate", "-config", configPath}, strings.NewReader(""), &stdout, &stderr)
	if code == 0 || !strings.Contains(stderr.String(), "is not a directory") {
		t.Fatalf("validate exit = %d, stderr = %q; want a not-a-directory error", code, stderr.String())
	}
}

func TestRunScanFailsForNonexistentTargetPath(t *testing.T) {
	configPath := writeMissingTargetConfig(t, t.TempDir(), "./does-not-exist")

	var stdout, stderr bytes.Buffer
	code := cli.Run([]string{"scan", "-config", configPath}, strings.NewReader(""), &stdout, &stderr)
	if code == 0 {
		t.Fatalf("scan exit code = 0 for a nonexistent target; stdout = %s", stdout.String())
	}
	if strings.Contains(stripANSI(stdout.String()), "PASS") {
		t.Fatalf("scan reported a passing section for a nonexistent target:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "does not exist") {
		t.Fatalf("stderr = %q, want a does-not-exist error", stderr.String())
	}
}

func TestRunValidateAcceptsExistingTargetPath(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "app"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	configPath := writeMissingTargetConfig(t, dir, "./app")

	var stdout, stderr bytes.Buffer
	if code := cli.Run([]string{"validate", "-config", configPath}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("validate exit = %d, stderr = %s", code, stderr.String())
	}
}
