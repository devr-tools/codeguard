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

	found := false
	for _, artifact := range report.Artifacts {
		if artifact.Kind != "performance_score" {
			continue
		}
		found = true
		if artifact.ID != "performance_score.python.repo" {
			t.Errorf("artifact ID = %q, want performance_score.python.repo", artifact.ID)
		}
		score := artifact.PerformanceScore
		if score == nil {
			t.Fatal("performance_score artifact has no payload")
		}
		if score.Score != 60 {
			t.Errorf("score = %d, want 60 (min(10*(5+1), 100))", score.Score)
		}
		if score.Signals != 2 {
			t.Errorf("signals = %d, want 2", score.Signals)
		}
		if len(score.Components) != 2 {
			t.Fatalf("components = %#v, want 2 entries", score.Components)
		}
		nPlusOne := score.Components[0]
		concat := score.Components[1]
		if nPlusOne.RuleID != "performance.n-plus-one-query" || nPlusOne.Weight != 5 || nPlusOne.Count != 1 || nPlusOne.Contribution != 5 {
			t.Errorf("unexpected n-plus-one component: %#v", nPlusOne)
		}
		if concat.RuleID != "performance.string-concat-in-loop" || concat.Weight != 1 || concat.Count != 1 || concat.Contribution != 1 {
			t.Errorf("unexpected string-concat component: %#v", concat)
		}
	}
	if !found {
		t.Fatalf("expected performance_score artifact, got %#v", report.Artifacts)
	}
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

	history := codeguard.LoadPerfScoreHistory(historyPath)
	if len(history) == 0 {
		t.Fatal("expected non-empty performance-score history")
	}
	for key, entries := range history {
		if len(entries) != 2 {
			t.Fatalf("history[%s] entries = %d, want 2", key, len(entries))
		}
		for _, entry := range entries {
			if entry.Timestamp == "" || entry.Score <= 0 || entry.Signals <= 0 || len(entry.Components) == 0 {
				t.Fatalf("incomplete history entry: %#v", entry)
			}
		}
	}
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
