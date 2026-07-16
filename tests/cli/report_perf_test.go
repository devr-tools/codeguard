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

// setupPerfHistoryReportFixture writes a Python fixture repo with an N+1
// pattern plus config, and seeds two performance scans so the report command
// has a recorded trend. It returns the config path.
func setupPerfHistoryReportFixture(t *testing.T, dir string) string {
	t.Helper()
	repo := filepath.Join(dir, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	source := `def fetch(items, cursor):
    rows = []
    for item in items:
        rows.append(cursor.execute("SELECT name FROM users WHERE id = ?"))
    return rows
`
	if err := os.WriteFile(filepath.Join(repo, "service.py"), []byte(source), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	cachePath := filepath.Join(dir, ".codeguard", "cache.json")
	configPath := filepath.Join(dir, "codeguard.json")
	config := fmt.Sprintf(`{
  "name": "report-perf-history",
  "targets": [{"name": "repo", "path": %q, "language": "python"}],
  "checks": {"performance": true, "quality": false, "design": false, "security": false, "prompts": false, "ci": false},
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

func TestRunReportPrintsPerfHistoryTrend(t *testing.T) {
	configPath := setupPerfHistoryReportFixture(t, t.TempDir())

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := cli.Run([]string{"report", "-perf-history", "-config", configPath}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("report exit code = %d, stderr = %s", code, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "performance_score.python.repo") {
		t.Fatalf("expected history key in output, got:\n%s", output)
	}
	if !strings.Contains(output, "score") || !strings.Contains(output, "(+0)") {
		t.Fatalf("expected score trend with delta, got:\n%s", output)
	}
	if !strings.Contains(output, "performance.n-plus-one-query=1") {
		t.Fatalf("expected component breakdown, got:\n%s", output)
	}
}

func TestRunReportHandlesMissingPerfHistory(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "codeguard.json")
	config := fmt.Sprintf(`{
  "name": "report-perf-empty",
  "targets": [{"name": "repo", "path": %q, "language": "python"}],
  "checks": {"performance": false, "quality": false, "design": false, "security": false, "prompts": false, "ci": false},
  "output": {"format": "json"},
  "cache": {"enabled": true, "path": %q}
}`, dir, filepath.Join(dir, ".codeguard", "cache.json"))
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := cli.Run([]string{"report", "-perf-history", "-config", configPath}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("report exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "no performance-score history recorded") {
		t.Fatalf("expected empty-history message, got: %s", stdout.String())
	}
}

func TestRunReportRejectsBothModeFlags(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := cli.Run([]string{"report", "-slop-history", "-perf-history"}, strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "only one mode flag") {
		t.Fatalf("expected single-mode error, got: %s", stderr.String())
	}
}
