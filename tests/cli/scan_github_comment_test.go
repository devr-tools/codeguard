package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/cli"
)

// The GitHub Action's sticky PR comment runs exactly this command. It used to
// fail config validation and print nothing, so the comment never posted.
func TestRunScanRendersGitHubCommentFormat(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}
	configPath := filepath.Join(dir, "codeguard.json")
	config := `{
  "name": "github-comment-scan",
  "targets": [{"name": "repo", "path": ".", "language": "go"}],
  "checks": {"quality": false, "design": false, "security": false, "prompts": false, "ci": false},
  "output": {"format": "text"},
  "cache": {"enabled": false}
}`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := cli.Run([]string{"scan", "-config", configPath, "-format", "github-comment"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("scan exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "## CodeGuard Fix Suggestions") {
		t.Fatalf("expected github-comment markdown on stdout, got:\n%s\nstderr:\n%s", stdout.String(), stderr.String())
	}
}
