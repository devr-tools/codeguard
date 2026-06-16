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

func writeMCPConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "codeguard.json")
	if err := os.WriteFile(configPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return configPath
}

func writePromptMCPFixture(t *testing.T) (string, string, string) {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "codeguard.json")
	promptPath := filepath.Join(dir, "prompts", "system.prompt")
	if err := os.MkdirAll(filepath.Dir(promptPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(promptPath, []byte("Keep prompts generic.\n"), 0o644); err != nil {
		t.Fatalf("write prompt: %v", err)
	}

	config := `{
  "name": "mcp-cli-test",
  "targets": [{"name": "repo", "path": "` + dir + `", "language": "go"}],
  "checks": {"quality": false, "design": false, "security": false, "prompts": true, "ci": false},
  "output": {"format": "json"}
}`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return configPath, promptPath, promptSecretPatchDiff()
}

func writeCancelableMCPFixture(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "codeguard.json")
	scriptPath := filepath.Join(dir, "slow-check.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nsleep 5\n"), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}
	config := `{
  "name": "` + name + `",
  "targets": [{"name": "repo", "path": "` + dir + `", "language": "go"}],
  "checks": {
    "quality": true,
    "design": false,
    "security": false,
    "prompts": false,
    "ci": false,
    "quality_rules": {
      "language_commands": {
        "go": [{"name": "slow-check", "command": "./slow-check.sh"}]
      }
    }
  },
  "output": {"format": "json"}
}`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return configPath
}

func runMCPServer(t *testing.T, configPath string, input string) []string {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := cli.Run([]string{"serve", "--mcp", "-config", configPath}, strings.NewReader(input), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d, stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	return nonEmptyLines(stdout.String())
}

func joinMCPMessages(t *testing.T, messages ...map[string]any) string {
	t.Helper()
	lines := make([]string, 0, len(messages))
	for _, msg := range messages {
		data, err := json.Marshal(msg)
		if err != nil {
			t.Fatalf("marshal message: %v", err)
		}
		lines = append(lines, string(data))
	}
	return strings.Join(lines, "\n") + "\n"
}

func decodeMCPLine(t *testing.T, line string, out any) {
	t.Helper()
	if err := json.Unmarshal([]byte(line), out); err != nil {
		t.Fatalf("decode line: %v line=%s", err, line)
	}
}

func nonEmptyLines(text string) []string {
	raw := strings.Split(text, "\n")
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func findResponseLineByID(t *testing.T, lines []string, want string) string {
	t.Helper()
	for _, line := range lines {
		var envelope struct {
			ID json.RawMessage `json:"id"`
		}
		decodeMCPLine(t, line, &envelope)
		if strings.TrimSpace(string(envelope.ID)) == want {
			return line
		}
	}
	t.Fatalf("response id %s not found in %q", want, lines)
	return ""
}

func containsTool(tools []struct {
	Name string `json:"name"`
}, name string) bool {
	for _, tool := range tools {
		if tool.Name == name {
			return true
		}
	}
	return false
}
