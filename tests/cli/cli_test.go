package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/cli"
)

func TestRunVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := cli.Run([]string{"version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if strings.TrimSpace(stdout.String()) == "" {
		t.Fatal("expected version output")
	}
}

func TestRunInitAndValidate(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "codeguard.json")

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if code := cli.Run([]string{"init", "-output", configPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("init exit code = %d, stderr = %s", code, stderr.String())
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("expected config file: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if code := cli.Run([]string{"validate", "-config", configPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("validate exit code = %d, stderr = %s", code, stderr.String())
	}
}
