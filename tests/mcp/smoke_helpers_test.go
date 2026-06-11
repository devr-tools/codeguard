package mcp_test

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/cli"
)

func TestMCPServeHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_MCP_HELPER_PROCESS") != "1" {
		return
	}
	args := os.Args
	idx := -1
	for i, arg := range args {
		if arg == "--" {
			idx = i
			break
		}
	}
	if idx == -1 || idx+1 >= len(args) {
		os.Exit(2)
	}
	configPath := args[idx+1]
	code := cli.Run([]string{"serve", "--mcp", "-config", configPath}, os.Stdin, os.Stdout, os.Stderr)
	os.Exit(code)
}

func setupPromptConfig(t *testing.T, dir string) map[string]string {
	t.Helper()
	configPath := filepath.Join(dir, "codeguard.json")
	promptPath := filepath.Join(dir, "prompts", "system.prompt")
	if err := os.MkdirAll(filepath.Dir(promptPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(promptPath, []byte("Keep prompts generic.\n"), 0o644); err != nil {
		t.Fatalf("write prompt: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(`{
  "name": "mcp-smoke-test",
  "targets": [{"name": "repo", "path": "`+dir+`", "language": "go"}],
  "checks": {"quality": false, "design": false, "security": false, "prompts": true, "ci": false},
  "output": {"format": "json"}
}`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return map[string]string{
		"__CONFIG_PATH__": configPath,
		"__DIFF__": strings.Join([]string{
			"diff --git a/prompts/system.prompt b/prompts/system.prompt",
			"index 6d6dd26..9a4f7f4 100644",
			"--- a/prompts/system.prompt",
			"+++ b/prompts/system.prompt",
			"@@ -1 +1 @@",
			"-Keep prompts generic.",
			"+Use ${OPENAI_API_KEY} for downstream calls.",
			"",
		}, "\n"),
	}
}

func setupCancelableScanConfig(t *testing.T, dir string) map[string]string {
	t.Helper()
	configPath := filepath.Join(dir, "codeguard.json")
	scriptPath := filepath.Join(dir, "slow-check.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nsleep 5\n"), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(`{
  "name": "mcp-scan-cancel-test",
  "targets": [{"name": "repo", "path": "`+dir+`", "language": "go"}],
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
}`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return map[string]string{"__CONFIG_PATH__": configPath}
}

func loadTranscript(t *testing.T, rel string, replacements map[string]string) string {
	t.Helper()
	data, err := os.ReadFile(rel)
	if err != nil {
		t.Fatalf("read transcript: %v", err)
	}
	text := string(data)
	for needle, value := range replacements {
		quotedValue, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("marshal replacement %s: %v", needle, err)
		}
		text = strings.ReplaceAll(text, `"`+needle+`"`, string(quotedValue))
	}
	return text
}

func runTranscriptThroughSubprocess(t *testing.T, configPath string, transcript string) ([]string, string) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=TestMCPServeHelperProcess", "--", configPath)
	cmd.Env = append(os.Environ(), "GO_WANT_MCP_HELPER_PROCESS=1")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("start subprocess: %v", err)
	}
	if _, err := io.WriteString(stdin, transcript); err != nil {
		t.Fatalf("write transcript: %v", err)
	}
	_ = stdin.Close()
	if err := cmd.Wait(); err != nil {
		t.Fatalf("wait subprocess: %v stdout=%s stderr=%s", err, stdout.String(), stderr.String())
	}
	return nonEmptyLines(stdout.String()), stderr.String()
}

func decodeLine(t *testing.T, line string, out any) {
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
		decodeLine(t, line, &envelope)
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
