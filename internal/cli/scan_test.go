package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/cli"
)

func TestRunScanRejectsInvalidMode(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := cli.Run([]string{"scan", "-mode", "sideways"}, strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), `invalid scan mode "sideways"`) {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestRunInteractiveScanUsesPromptedBaseRef(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "codeguard.json")
	config := `{
  "name": "interactive-scan",
  "targets": [{"name": "repo", "path": ".", "language": "go"}],
  "checks": {"quality": false, "design": false, "security": false, "prompts": false, "ci": false},
  "output": {"format": "text"}
}`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	input := strings.NewReader(configPath + "\ndiff\norigin/main\n")

	code := cli.Run([]string{"scan", "-interactive"}, input, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("scan exit code = %d, stderr = %s", code, stderr.String())
	}
	rendered := stripANSI(stdout.String())
	if !strings.Contains(rendered, "Base Ref: origin/main") {
		t.Fatalf("expected prompted base ref in output, got:\n%s", rendered)
	}
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(value string) string {
	return ansiPattern.ReplaceAllString(value, "")
}
