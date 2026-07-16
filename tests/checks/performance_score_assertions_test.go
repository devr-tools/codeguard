package checks_test

import (
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

type expectedPerformanceComponent struct {
	ruleID       string
	weight       int
	count        int
	contribution int
}

func assertPerformanceScoreArtifact(t *testing.T, report codeguard.Report) {
	t.Helper()
	found := false
	for _, artifact := range report.Artifacts {
		if artifact.Kind != "performance_score" {
			continue
		}
		found = true
		if artifact.ID != "performance_score.python.repo" {
			t.Errorf("artifact ID = %q, want performance_score.python.repo", artifact.ID)
		}
		assertPerformanceScorePayload(t, artifact.PerformanceScore)
	}
	if !found {
		t.Fatalf("expected performance_score artifact, got %#v", report.Artifacts)
	}
}

func assertPerformanceScorePayload(t *testing.T, score *codeguard.PerformanceScoreArtifact) {
	t.Helper()
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
	assertPerformanceScoreComponent(t, score.Components[0], expectedPerformanceComponent{ruleID: "performance.n-plus-one-query", weight: 5, count: 1, contribution: 5})
	assertPerformanceScoreComponent(t, score.Components[1], expectedPerformanceComponent{ruleID: "performance.string-concat-in-loop", weight: 1, count: 1, contribution: 1})
}

func assertPerformanceScoreComponent(t *testing.T, component codeguard.SlopScoreComponent, want expectedPerformanceComponent) {
	t.Helper()
	if component.RuleID != want.ruleID || component.Weight != want.weight || component.Count != want.count || component.Contribution != want.contribution {
		t.Errorf("unexpected component: %#v", component)
	}
}

func assertPerformanceHistoryEntries(t *testing.T, history map[string][]codeguard.PerformanceHistoryEntry) {
	t.Helper()
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
