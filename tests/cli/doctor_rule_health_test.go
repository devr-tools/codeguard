package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/cli"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

func writeRuleHealthConfig(t *testing.T, dir string, waivers string, cachePath string) string {
	t.Helper()
	configPath := filepath.Join(dir, "codeguard.json")
	config := `{
  "name": "doctor-rule-health",
  "targets": [{"name": "repo", "path": "` + dir + `", "language": "go"}],
  "checks": {"quality": false, "design": false, "security": false, "prompts": false, "ci": false},
  "output": {"format": "text"},
  "cache": {"path": "` + cachePath + `"},
  "waivers": [` + waivers + `]
}`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return configPath
}

func runRuleHealthDoctor(t *testing.T, configPath string) string {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := cli.Run([]string{"doctor", "-config", configPath}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0 (rule-health issues warn, not fail), got %d, stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	return stdout.String()
}

func TestRunDoctorWaiverHealth(t *testing.T) {
	cases := []struct {
		name        string
		waivers     string
		wantLine    string
		wantMessage string
	}{
		{
			name:        "dead_waiver_warns",
			waivers:     `{"rule": "quality.not-a-real-rule"}`,
			wantLine:    "[WARN] waiver:quality.not-a-real-rule:",
			wantMessage: "matches no catalog rule",
		},
		{
			name:        "retired_performance_waiver_gets_migration_hint",
			waivers:     `{"rule": "quality.n-plus-one-query"}`,
			wantLine:    "[WARN] waiver:quality.n-plus-one-query:",
			wantMessage: "rule moved to the performance section as performance.n-plus-one-query",
		},
		{
			name:        "expired_waiver_warns",
			waivers:     `{"rule": "quality.gofmt", "expires_on": "2020-01-01"}`,
			wantLine:    "[WARN] waiver:quality.gofmt:",
			wantMessage: "expired on 2020-01-01",
		},
		{
			name:        "healthy_waivers_pass",
			waivers:     `{"rule": "quality.gofmt", "expires_on": "2999-01-01"}, {"rule": "*", "path": "vendored/**"}`,
			wantLine:    "[PASS] waivers:",
			wantMessage: "all 2 waiver(s) match catalog rules and are unexpired",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			configPath := writeRuleHealthConfig(t, dir, tc.waivers, filepath.Join(dir, "cache", "scan.json"))
			out := runRuleHealthDoctor(t, configPath)
			if !strings.Contains(out, tc.wantLine) || !strings.Contains(out, tc.wantMessage) {
				t.Fatalf("expected doctor output containing %q and %q, got: %s", tc.wantLine, tc.wantMessage, out)
			}
		})
	}
}

func TestRunDoctorFlagsHighSuppressionRuleFromLastScan(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "cache", "scan.json")
	configPath := writeRuleHealthConfig(t, dir, "", cachePath)
	appendRuleStatsHistory(t, cachePath, core.RuleStatsHistoryEntry{
		Timestamp: "2026-07-01T00:00:00Z",
		Rules: []core.RuleStatsEntry{
			{RuleID: "quality.noisy-rule", Emitted: 1, WaiverSuppressed: 9, SuppressionRatio: 0.9},
			{RuleID: "quality.quiet-rule", Emitted: 9, BaselineSuppressed: 1, SuppressionRatio: 0.1},
			{RuleID: "quality.low-volume-rule", Emitted: 1, InlineSuppressed: 2, SuppressionRatio: 0.667},
		},
	})

	out := runRuleHealthDoctor(t, configPath)
	if !strings.Contains(out, "[WARN] rule-health:quality.noisy-rule:") ||
		!strings.Contains(out, "9 of 10 findings suppressed in the last scan; consider tuning or disabling quality.noisy-rule") {
		t.Fatalf("expected high-suppression warning, got: %s", out)
	}
	if strings.Contains(out, "rule-health:quality.quiet-rule") {
		t.Fatalf("did not expect low-ratio rule to be flagged, got: %s", out)
	}
	if strings.Contains(out, "rule-health:quality.low-volume-rule") {
		t.Fatalf("did not expect below-minimum-volume rule to be flagged, got: %s", out)
	}
}

// TestRunDoctorRuleHealthUsesLatestScan proves doctor reads the most recent
// history entry: an older noisy scan followed by a healthy one must pass.
func TestRunDoctorRuleHealthUsesLatestScan(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "cache", "scan.json")
	configPath := writeRuleHealthConfig(t, dir, "", cachePath)
	appendRuleStatsHistory(t, cachePath, core.RuleStatsHistoryEntry{
		Timestamp: "2026-06-30T00:00:00Z",
		Rules:     []core.RuleStatsEntry{{RuleID: "quality.noisy-rule", Emitted: 1, WaiverSuppressed: 9, SuppressionRatio: 0.9}},
	})
	appendRuleStatsHistory(t, cachePath, core.RuleStatsHistoryEntry{
		Timestamp: "2026-07-01T00:00:00Z",
		Rules:     []core.RuleStatsEntry{{RuleID: "quality.noisy-rule", Emitted: 10, SuppressionRatio: 0}},
	})

	out := runRuleHealthDoctor(t, configPath)
	if !strings.Contains(out, "[PASS] rule-health: no rule exceeded the suppression threshold in the last scan") {
		t.Fatalf("expected rule-health pass from latest scan, got: %s", out)
	}
	if strings.Contains(out, "[WARN] rule-health:") {
		t.Fatalf("did not expect stale-scan warning, got: %s", out)
	}
}

// TestRunDoctorRuleHealthSilentWithoutHistory locks in that doctor stays quiet
// about rule health when no scan has recorded stats yet.
func TestRunDoctorRuleHealthSilentWithoutHistory(t *testing.T) {
	dir := t.TempDir()
	configPath := writeRuleHealthConfig(t, dir, "", filepath.Join(dir, "cache", "scan.json"))
	out := runRuleHealthDoctor(t, configPath)
	if strings.Contains(out, "rule-health") {
		t.Fatalf("expected no rule-health output without history, got: %s", out)
	}
}

func appendRuleStatsHistory(t *testing.T, cachePath string, entry core.RuleStatsHistoryEntry) {
	t.Helper()
	historyPath := runnersupport.RuleStatsHistoryPathForBase(cachePath)
	if historyPath == "" {
		t.Fatal("expected non-empty rule-stats history path")
	}
	runnersupport.AppendRuleStatsHistory(historyPath, entry, 0)
}
