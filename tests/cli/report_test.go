package cli_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/cli"
	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestRunReportRequiresModeFlag(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := cli.Run([]string{"report"}, strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "-slop-history") {
		t.Fatalf("expected mode flag hint, got: %s", stderr.String())
	}
}

// setupSlopHistoryReportFixture writes a Go fixture repo plus config and
// seeds two scans so the report command has a recorded trend. It returns the
// config path.
func setupSlopHistoryReportFixture(t *testing.T, dir string) string {
	t.Helper()
	repo := filepath.Join(dir, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	source := `package sample

func Run() error {
	err := doThing()
	_ = err
	return nil
}

func doThing() error { return nil }
`
	if err := os.WriteFile(filepath.Join(repo, "service.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	cachePath := filepath.Join(dir, ".codeguard", "cache.json")
	configPath := filepath.Join(dir, "codeguard.json")
	config := fmt.Sprintf(`{
  "name": "report-history",
  "targets": [{"name": "repo", "path": %q, "language": "go"}],
  "checks": {"quality": true, "design": false, "security": false, "prompts": false, "ci": false},
  "output": {"format": "json"},
  "cache": {"enabled": true, "path": %q}
}`, repo, cachePath)
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := codeguard.LoadConfigFile(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	for i := 0; i < 2; i++ {
		if _, err := codeguard.Run(context.Background(), cfg); err != nil {
			t.Fatalf("scan %d: %v", i, err)
		}
	}
	return configPath
}

func TestRunReportPrintsSlopHistoryTrend(t *testing.T) {
	configPath := setupSlopHistoryReportFixture(t, t.TempDir())

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := cli.Run([]string{"report", "-slop-history", "-config", configPath}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("report exit code = %d, stderr = %s", code, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "slop_score.go.repo") {
		t.Fatalf("expected history key in output, got:\n%s", output)
	}
	if !strings.Contains(output, "score") || !strings.Contains(output, "(+0)") {
		t.Fatalf("expected score trend with delta, got:\n%s", output)
	}
	if !strings.Contains(output, "quality.ai.swallowed-error=1") {
		t.Fatalf("expected component breakdown, got:\n%s", output)
	}
}

func TestRunReportHandlesMissingHistory(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "codeguard.json")
	config := fmt.Sprintf(`{
  "name": "report-empty",
  "targets": [{"name": "repo", "path": %q, "language": "go"}],
  "checks": {"quality": false, "design": false, "security": false, "prompts": false, "ci": false},
  "output": {"format": "json"},
  "cache": {"enabled": true, "path": %q}
}`, dir, filepath.Join(dir, ".codeguard", "cache.json"))
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := cli.Run([]string{"report", "-slop-history", "-config", configPath}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("report exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "no slop-score history recorded") {
		t.Fatalf("expected empty-history message, got: %s", stdout.String())
	}
}
