package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/cli"
)

func runValidatePatchFailOn(t *testing.T, extra ...string) (int, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "prompts"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "prompts", "system.prompt"), []byte("Keep prompts generic.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// A text-format config: -fail-on governs the exit code whatever the
	// report format.
	configPath := filepath.Join(dir, "codeguard.yaml")
	config := "name: patch-fail-on\ntargets:\n  - name: repo\n    path: " + dir + "\n    language: go\nchecks:\n  quality: false\n  design: false\n  security: false\n  prompts: true\n  ci: false\noutput:\n  format: text\n"
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	args := append([]string{"validate-patch", "-config", configPath}, extra...)
	var stdout, stderr bytes.Buffer
	code := cli.Run(args, strings.NewReader(promptSecretPatchDiff()), &stdout, &stderr)
	return code, stderr.String()
}

func TestValidatePatchFailOnFlag(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantCode int
	}{
		{name: "default fails on a failing section", wantCode: 1},
		{name: "fail-on none exits zero", args: []string{"-fail-on", "none"}, wantCode: 0},
		{name: "fail-on is case-insensitive", args: []string{"-fail-on", "NONE"}, wantCode: 0},
		{name: "fail-on fail keeps failing", args: []string{"-fail-on", "fail"}, wantCode: 1},
		{name: "fail-on wins over -set output.fail_on", args: []string{"-set", "output.fail_on=none", "-fail-on", "fail"}, wantCode: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stderr := runValidatePatchFailOn(t, tt.args...)
			if code != tt.wantCode {
				t.Fatalf("exit = %d, want %d; stderr=%s", code, tt.wantCode, stderr)
			}
		})
	}
}

func TestValidatePatchRejectsInvalidFailOn(t *testing.T) {
	code, stderr := runValidatePatchFailOn(t, "-fail-on", "sometimes")
	if code == 0 {
		t.Fatal("invalid -fail-on must be rejected")
	}
	if !strings.Contains(stderr, `invalid -fail-on "sometimes": must be one of`) {
		t.Fatalf("stderr = %s", stderr)
	}
}
