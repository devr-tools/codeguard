package support_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

func TestRuleStatsHistoryPathForBase(t *testing.T) {
	cases := []struct {
		name string
		base string
		want string
	}{
		{"empty", "", ""},
		{"whitespace", "   ", ""},
		{"json_extension", ".codeguard/cache.json", ".codeguard/cache.rule-stats-history.json"},
		{"no_extension", ".codeguard/cache", ".codeguard/cache.rule-stats-history"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := runnersupport.RuleStatsHistoryPathForBase(tc.base); got != tc.want {
				t.Fatalf("RuleStatsHistoryPathForBase(%q) = %q, want %q", tc.base, got, tc.want)
			}
		})
	}
}

func TestRuleStatsHistoryRoundTripAndCap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.rule-stats-history.json")

	if got := runnersupport.LoadRuleStatsHistory(path); len(got) != 0 {
		t.Fatalf("expected empty history for missing file, got %#v", got)
	}

	for i, ruleID := range []string{"first.rule", "second.rule", "third.rule"} {
		runnersupport.AppendRuleStatsHistory(path, core.RuleStatsHistoryEntry{
			Timestamp: fmt.Sprintf("2026-07-%02dT00:00:00Z", i+1),
			Rules:     []core.RuleStatsEntry{{RuleID: ruleID, Emitted: 1}},
		}, 2)
	}

	history := runnersupport.LoadRuleStatsHistory(path)
	if len(history) != 2 {
		t.Fatalf("expected history capped at 2 entries, got %d: %#v", len(history), history)
	}
	if history[0].Rules[0].RuleID != "second.rule" || history[1].Rules[0].RuleID != "third.rule" {
		t.Fatalf("expected oldest entry evicted, got %#v", history)
	}
	if history[1].Timestamp != "2026-07-03T00:00:00Z" {
		t.Fatalf("unexpected latest timestamp %q", history[1].Timestamp)
	}
}
