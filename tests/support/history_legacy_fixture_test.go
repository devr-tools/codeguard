package support_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

// testdata/history_v1 was written by the per-kind history persistence code
// that predates the generic keyedHistory/listHistory store. These tests pin
// that existing history files still load and that the store still writes the
// same bytes for the same observations.

const legacyHistoryDir = "testdata/history_v1"

var legacyComponents = []core.SlopScoreComponent{{RuleID: "quality.ai.narrative-comment", Count: 2, Weight: 1, Contribution: 2}}

func TestLegacyKeyedHistoryFixturesLoad(t *testing.T) {
	slop := runnersupport.LoadSlopHistory(filepath.Join(legacyHistoryDir, "slop-history.json"))
	if len(slop) != 2 || len(slop["ai_slop:repo"]) != 2 || len(slop["ai_slop:other"]) != 1 {
		t.Fatalf("slop history = %#v, want 2 keys with 2 and 1 entries", slop)
	}
	if got := slop["ai_slop:repo"][0]; got.Score != 20 || got.Signals != 2 || len(got.Components) != 1 || got.Components[0] != legacyComponents[0] {
		t.Fatalf("first slop entry = %#v", got)
	}
	if got := slop["ai_slop:repo"][1]; got.Timestamp != "2026-10-02T00:00:00Z" || got.Score != 10 {
		t.Fatalf("latest slop entry = %#v", got)
	}
}

func TestLegacyKeyedPerfAndLegibilityHistoryFixturesLoad(t *testing.T) {
	perf := runnersupport.LoadPerfScoreHistory(filepath.Join(legacyHistoryDir, "perf-history.json"))
	if entries := perf["performance_score:repo"]; len(entries) != 2 || entries[0].Score != 30 || entries[1].Score != 40 || entries[1].Signals != 4 {
		t.Fatalf("perf history = %#v", perf)
	}

	legibility := runnersupport.LoadLegibilityHistory(filepath.Join(legacyHistoryDir, "legibility-history.json"))
	entries := legibility["repo_legibility:repo"]
	if len(entries) != 2 || entries[0].Score != 70 || entries[1].Score != 75 {
		t.Fatalf("legibility history = %#v", legibility)
	}
	if len(entries[0].Components) != 1 || entries[0].Components[0].Detail != "AGENTS.md present" {
		t.Fatalf("legibility components = %#v", entries[0].Components)
	}
}

func TestLegacyListHistoryFixturesLoad(t *testing.T) {
	stats := runnersupport.LoadRuleStatsHistory(filepath.Join(legacyHistoryDir, "rule-stats-history.json"))
	if len(stats) != 2 || stats[0].Rules[0].WaiverSuppressed != 1 || stats[1].Rules[0].BaselineSuppressed != 4 {
		t.Fatalf("rule stats history = %#v", stats)
	}

	audits := runnersupport.LoadWaiverAuditHistory(filepath.Join(legacyHistoryDir, "waiver-audit-history.json"))
	if len(audits) != 1 {
		t.Fatalf("waiver audit history = %#v", audits)
	}
	audit := audits[0]
	if audit.CodeGuardVersion != "1.2.3" || audit.ConfigFingerprint != "abc123" || len(audit.Waivers) != 1 {
		t.Fatalf("waiver audit entry = %#v", audit)
	}
	if waiver := audit.Waivers[0]; waiver.Rule != "quality.file-too-long" || waiver.MatchedFindings != 1 || len(waiver.MatchedFingerprints) != 1 {
		t.Fatalf("waiver = %#v", waiver)
	}
}

// TestHistoryStoreWritesLegacyBytes replays the observations that produced
// the fixtures through the current store and requires byte-identical files.
func TestHistoryStoreWritesLegacyBytes(t *testing.T) {
	dir := t.TempDir()
	path := func(name string) string { return filepath.Join(dir, name) }
	mustNoErr := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	appendSlop := func(key string, entry core.SlopHistoryEntry) {
		_, _, err := runnersupport.AppendSlopHistory(path("slop-history.json"), key, entry, 0)
		mustNoErr(err)
	}
	appendSlop("ai_slop:repo", core.SlopHistoryEntry{Timestamp: "2026-10-01T00:00:00Z", Score: 20, Signals: 2, Components: legacyComponents})
	appendSlop("ai_slop:repo", core.SlopHistoryEntry{Timestamp: "2026-10-02T00:00:00Z", Score: 10, Signals: 1})
	appendSlop("ai_slop:other", core.SlopHistoryEntry{Timestamp: "2026-10-02T00:00:00Z", Score: 0, Signals: 0})

	_, _, err := runnersupport.AppendPerfScoreHistory(path("perf-history.json"), "performance_score:repo", core.PerformanceHistoryEntry{Timestamp: "2026-10-01T00:00:00Z", Score: 30, Signals: 3, Components: legacyComponents}, 0)
	mustNoErr(err)
	_, _, err = runnersupport.AppendPerfScoreHistory(path("perf-history.json"), "performance_score:repo", core.PerformanceHistoryEntry{Timestamp: "2026-10-02T00:00:00Z", Score: 40, Signals: 4}, 0)
	mustNoErr(err)

	_, _, err = runnersupport.AppendLegibilityHistory(path("legibility-history.json"), "repo_legibility:repo", core.LegibilityHistoryEntry{Timestamp: "2026-10-01T00:00:00Z", Score: 70, Components: []core.RepoLegibilityComponent{{Label: "agent docs", Score: 7, Max: 10, Detail: "AGENTS.md present"}}}, 0)
	mustNoErr(err)
	_, _, err = runnersupport.AppendLegibilityHistory(path("legibility-history.json"), "repo_legibility:repo", core.LegibilityHistoryEntry{Timestamp: "2026-10-02T00:00:00Z", Score: 75}, 0)
	mustNoErr(err)

	mustNoErr(runnersupport.AppendRuleStatsHistory(path("rule-stats-history.json"), core.RuleStatsHistoryEntry{Timestamp: "2026-10-01T00:00:00Z", Rules: []core.RuleStatsEntry{{RuleID: "quality.file-too-long", Emitted: 3, WaiverSuppressed: 1}}}, 0))
	mustNoErr(runnersupport.AppendRuleStatsHistory(path("rule-stats-history.json"), core.RuleStatsHistoryEntry{Timestamp: "2026-10-02T00:00:00Z", Rules: []core.RuleStatsEntry{{RuleID: "quality.file-too-long", Emitted: 2, BaselineSuppressed: 4}}}, 0))

	mustNoErr(runnersupport.AppendWaiverAuditHistory(path("waiver-audit-history.json"), core.WaiverAuditHistoryEntry{Timestamp: "2026-10-01T00:00:00Z", CodeGuardVersion: "1.2.3", ConfigFingerprint: "abc123", ScanMode: "full", Waivers: []core.WaiverAuditEntry{{Index: 0, Rule: "quality.file-too-long", Path: "a.go", Reason: "legacy", ExpiresOn: "2027-01-01", Status: "used", MatchedFindings: 1, MatchedFingerprints: []string{"fp1"}}}}, 0))

	for _, name := range []string{"slop-history.json", "perf-history.json", "legibility-history.json", "rule-stats-history.json", "waiver-audit-history.json"} {
		want, err := os.ReadFile(filepath.Join(legacyHistoryDir, name))
		mustNoErr(err)
		got, err := os.ReadFile(path(name))
		mustNoErr(err)
		if !bytes.Equal(got, want) {
			t.Errorf("%s differs from legacy fixture:\n got: %s\nwant: %s", name, got, want)
		}
	}
}

func TestAppendHistoryReportsWriteFailure(t *testing.T) {
	// A regular file where the history's parent directory should be makes
	// MkdirAll fail, so the write error must surface instead of vanishing.
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(blocker, "slop-history.json")
	if _, _, err := runnersupport.AppendSlopHistory(path, "ai_slop:repo", core.SlopHistoryEntry{Score: 1}, 0); err == nil {
		t.Fatal("AppendSlopHistory() error = nil, want write failure")
	}
	if err := runnersupport.AppendRuleStatsHistory(path, core.RuleStatsHistoryEntry{}, 0); err == nil {
		t.Fatal("AppendRuleStatsHistory() error = nil, want write failure")
	}
	diagnostic := runnersupport.HistoryWriteDiagnostic("slop", path, os.ErrPermission)
	if diagnostic.ID != runnersupport.HistoryWriteDiagnosticID || diagnostic.Level != "warn" || !diagnostic.Operational || diagnostic.Path != path {
		t.Fatalf("diagnostic = %#v, want warn-level operational %s", diagnostic, runnersupport.HistoryWriteDiagnosticID)
	}
}
