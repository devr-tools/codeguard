package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/cli"
)

func TestRunDoctorFailsWhenDesignCommandIsMissing(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "codeguard.json")
	config := `{
  "name": "doctor-design-command",
  "targets": [{"name": "web", "path": "` + dir + `", "language": "typescript"}],
  "checks": {
    "quality": false,
    "design": true,
    "security": false,
    "prompts": false,
    "ci": false,
    "design_rules": {
      "language_commands": {
        "typescript": [
          {"name": "depcruise", "command": "definitely-not-installed-design-tool"}
        ]
      }
    }
  },
  "output": {"format": "text"}
}`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := cli.Run([]string{"doctor", "-config", configPath}, strings.NewReader(""), &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected doctor failure, stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "design:web:depcruise") {
		t.Fatalf("expected design command doctor check, got: %s", stdout.String())
	}
}
