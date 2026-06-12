package checks_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func slopHistoryTestConfig(dir string, name string) codeguard.Config {
	cfg := qualityAITestConfig(dir, name)
	enabled := true
	cfg.Cache = codeguard.CacheConfig{
		Enabled: &enabled,
		Path:    filepath.Join(dir, ".codeguard", "cache.json"),
	}
	return cfg
}

func writeSlopFixture(t *testing.T, dir string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, "service.go"), `package sample

func Run() error {
	err := doThing()
	_ = err
	return nil
}

func doThing() error { return nil }
`)
}

func TestSlopScoreHistoryRecordsTrendAndDelta(t *testing.T) {
	dir := t.TempDir()
	writeSlopFixture(t, dir)
	cfg := slopHistoryTestConfig(dir, "quality-ai-history")

	first, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	firstArtifact := findSlopScoreArtifact(t, first)
	if firstArtifact.PreviousScore != nil || firstArtifact.Delta != nil {
		t.Fatalf("first scan should have no previous score, got %#v", firstArtifact)
	}

	historyPath := codeguard.SlopHistoryPath(cfg)
	if _, err := os.Stat(historyPath); err != nil {
		t.Fatalf("expected history file at %s: %v", historyPath, err)
	}

	second, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	secondArtifact := findSlopScoreArtifact(t, second)
	if secondArtifact.PreviousScore == nil || secondArtifact.Delta == nil {
		t.Fatalf("second scan should report previous score and delta, got %#v", secondArtifact)
	}
	if *secondArtifact.PreviousScore != firstArtifact.Score {
		t.Fatalf("previous score = %d, want %d", *secondArtifact.PreviousScore, firstArtifact.Score)
	}
	if *secondArtifact.Delta != secondArtifact.Score-firstArtifact.Score {
		t.Fatalf("delta = %d, want %d", *secondArtifact.Delta, secondArtifact.Score-firstArtifact.Score)
	}

	history := codeguard.LoadSlopHistory(historyPath)
	if len(history) == 0 {
		t.Fatal("expected non-empty slop history")
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

func TestSlopScoreHistoryHonorsToggle(t *testing.T) {
	dir := t.TempDir()
	writeSlopFixture(t, dir)
	cfg := slopHistoryTestConfig(dir, "quality-ai-history-toggle")
	disabled := false
	cfg.Checks.QualityRules.AIChecks.SlopHistory = &disabled

	if _, err := codeguard.Run(context.Background(), cfg); err != nil {
		t.Fatalf("run: %v", err)
	}
	if _, err := os.Stat(codeguard.SlopHistoryPath(cfg)); !os.IsNotExist(err) {
		t.Fatalf("expected no history file when disabled, stat err = %v", err)
	}
}

func TestSlopScoreHistoryCapsEntries(t *testing.T) {
	dir := t.TempDir()
	writeSlopFixture(t, dir)
	cfg := slopHistoryTestConfig(dir, "quality-ai-history-cap")
	cfg.Checks.QualityRules.AIChecks.SlopHistoryLimit = 2

	for i := 0; i < 3; i++ {
		if _, err := codeguard.Run(context.Background(), cfg); err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
	}

	history := codeguard.LoadSlopHistory(codeguard.SlopHistoryPath(cfg))
	for key, entries := range history {
		if len(entries) != 2 {
			t.Fatalf("history[%s] entries = %d, want capped at 2", key, len(entries))
		}
	}
}

func findSlopScoreArtifact(t *testing.T, report codeguard.Report) *codeguard.SlopScoreArtifact {
	t.Helper()
	for _, artifact := range report.Artifacts {
		if artifact.Kind == "slop_score" && artifact.SlopScore != nil {
			return artifact.SlopScore
		}
	}
	t.Fatalf("expected slop_score artifact, got %#v", report.Artifacts)
	return nil
}
