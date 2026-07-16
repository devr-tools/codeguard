package checks_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// legibilityHistoryConfig enables the cache so the trend file is written next
// to it, mirroring the slop/perf history plumbing.
func legibilityHistoryConfig(t *testing.T, repo string, cacheDir string, name string) codeguard.Config {
	t.Helper()
	cfg := agentContextTestConfig(repo, name)
	on := true
	cfg.Cache.Enabled = &on
	cfg.Cache.Path = filepath.Join(cacheDir, "cache.json")
	return cfg
}

func TestLegibilityHistoryPersistsTrendAndAnnotatesDelta(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeLegibleRepoFixture(t, repo)
	cfg := legibilityHistoryConfig(t, repo, dir, "legibility-history")

	first, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	if artifact := requireRepoLegibilityArtifact(t, first); artifact.RepoLegibility.PreviousScore != nil {
		t.Fatalf("first scan must not report a previous score, got %+v", artifact.RepoLegibility)
	}

	second, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	artifact := requireRepoLegibilityArtifact(t, second)
	if artifact.RepoLegibility.PreviousScore == nil || artifact.RepoLegibility.Delta == nil {
		t.Fatalf("second scan should carry previous_score and delta: %+v", artifact.RepoLegibility)
	}
	if *artifact.RepoLegibility.PreviousScore != artifact.RepoLegibility.Score || *artifact.RepoLegibility.Delta != 0 {
		t.Fatalf("unchanged repo should have zero delta: %+v", artifact.RepoLegibility)
	}

	path := codeguard.LegibilityHistoryPath(cfg)
	if !strings.HasSuffix(path, ".legibility-history.json") {
		t.Fatalf("unexpected history path: %q", path)
	}
	history := codeguard.LoadLegibilityHistory(path)
	entries := history[artifact.ID]
	if len(entries) != 2 {
		t.Fatalf("history entries = %d, want 2 (keys: %v)", len(entries), historyKeys(history))
	}
	if entries[0].Score != artifact.RepoLegibility.Score || len(entries[0].Components) == 0 {
		t.Fatalf("history entry should carry score and components: %+v", entries[0])
	}
}

func TestLegibilityHistoryDisabledByToggle(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeLegibleRepoFixture(t, repo)
	cfg := legibilityHistoryConfig(t, repo, dir, "legibility-history-off")
	off := false
	cfg.Checks.ContextRules.LegibilityHistory = &off

	if _, err := codeguard.Run(context.Background(), cfg); err != nil {
		t.Fatalf("run: %v", err)
	}
	if _, err := os.Stat(codeguard.LegibilityHistoryPath(cfg)); !os.IsNotExist(err) {
		t.Fatalf("history file should not exist when legibility_history=false, stat err: %v", err)
	}
}

func historyKeys(history map[string][]codeguard.LegibilityHistoryEntry) []string {
	keys := make([]string, 0, len(history))
	for key := range history {
		keys = append(keys, key)
	}
	return keys
}
