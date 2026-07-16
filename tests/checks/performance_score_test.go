package checks_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// writePerformanceScoreFixture produces one N+1 finding (family weight 5)
// and one string-concat-in-loop finding (family weight 1), so the expected
// score is min(10*(5+1), 100) = 60 with 2 signals.
func writePerformanceScoreFixture(t *testing.T, dir string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, "service.py"), `def render(items, cursor):
    out = ""
    for item in items:
        row = cursor.execute("SELECT name FROM users WHERE id = ?")
        out += str(row)
    return out
`)
}

func performanceScoreConfig(dir string, name string) codeguard.Config {
	cfg := performanceConfig(name, dir, "python")
	enabled := true
	cfg.Cache = codeguard.CacheConfig{
		Enabled: &enabled,
		Path:    filepath.Join(dir, ".codeguard", "cache.json"),
	}
	return cfg
}

func runPerformanceScoreScan(t *testing.T, cfg codeguard.Config, label string) *codeguard.PerformanceScoreArtifact {
	t.Helper()
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("%s run: %v", label, err)
	}
	return findPerformanceScoreArtifact(t, report)
}

func findPerformanceScoreArtifact(t *testing.T, report codeguard.Report) *codeguard.PerformanceScoreArtifact {
	t.Helper()
	for _, artifact := range report.Artifacts {
		if artifact.Kind == "performance_score" && artifact.PerformanceScore != nil {
			return artifact.PerformanceScore
		}
	}
	t.Fatalf("expected performance_score artifact, got %#v", report.Artifacts)
	return nil
}

func TestPerformanceScoreArtifactComputesWeightedScore(t *testing.T) {
	dir := t.TempDir()
	writePerformanceScoreFixture(t, dir)
	cfg := performanceScoreConfig(dir, "performance-score")

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertPerformanceScoreArtifact(t, report)
}

func TestPerformanceScoreAbsentWithoutFindings(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "service.py"), `def render(items):
    return [item for item in items]
`)

	report, err := codeguard.Run(context.Background(), performanceScoreConfig(dir, "performance-score-clean"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, artifact := range report.Artifacts {
		if artifact.Kind == "performance_score" {
			t.Fatalf("expected no performance_score artifact on a clean target, got %#v", artifact)
		}
	}
}

func TestPerformanceScoreHistoryRecordsTrendAndDelta(t *testing.T) {
	dir := t.TempDir()
	writePerformanceScoreFixture(t, dir)
	cfg := performanceScoreConfig(dir, "performance-score-history")

	first := runPerformanceScoreScan(t, cfg, "first")
	if first.PreviousScore != nil || first.Delta != nil {
		t.Fatalf("first scan should have no previous score, got %#v", first)
	}

	historyPath := codeguard.PerfScoreHistoryPath(cfg)
	if _, err := os.Stat(historyPath); err != nil {
		t.Fatalf("expected history file at %s: %v", historyPath, err)
	}

	second := runPerformanceScoreScan(t, cfg, "second")
	if second.PreviousScore == nil || second.Delta == nil {
		t.Fatalf("second scan should report previous score and delta, got %#v", second)
	}
	if *second.PreviousScore != first.Score {
		t.Fatalf("previous score = %d, want %d", *second.PreviousScore, first.Score)
	}
	if *second.Delta != second.Score-first.Score {
		t.Fatalf("delta = %d, want %d", *second.Delta, second.Score-first.Score)
	}

	assertPerformanceHistoryEntries(t, codeguard.LoadPerfScoreHistory(historyPath))
}

func TestPerformanceScoreHistoryHonorsToggle(t *testing.T) {
	dir := t.TempDir()
	writePerformanceScoreFixture(t, dir)
	cfg := performanceScoreConfig(dir, "performance-score-history-toggle")
	disabled := false
	cfg.Checks.PerformanceRules.ScoreHistory = &disabled

	if _, err := codeguard.Run(context.Background(), cfg); err != nil {
		t.Fatalf("run: %v", err)
	}
	if _, err := os.Stat(codeguard.PerfScoreHistoryPath(cfg)); !os.IsNotExist(err) {
		t.Fatalf("expected no history file when disabled, stat err = %v", err)
	}
}

func TestPerformanceScoreHistoryCapsEntries(t *testing.T) {
	dir := t.TempDir()
	writePerformanceScoreFixture(t, dir)
	cfg := performanceScoreConfig(dir, "performance-score-history-cap")
	cfg.Checks.PerformanceRules.ScoreHistoryLimit = 2

	for i := 0; i < 3; i++ {
		if _, err := codeguard.Run(context.Background(), cfg); err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
	}

	history := codeguard.LoadPerfScoreHistory(codeguard.PerfScoreHistoryPath(cfg))
	for key, entries := range history {
		if len(entries) != 2 {
			t.Fatalf("history[%s] entries = %d, want capped at 2", key, len(entries))
		}
	}
}
