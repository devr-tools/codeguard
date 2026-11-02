package support

import (
	"strings"
	"time"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// DefaultRuleStatsHistoryLimit caps how many scans of rule stats are retained,
// mirroring the slop-history cap.
const DefaultRuleStatsHistoryLimit = 100

var ruleStatsHistory = listHistory[core.RuleStatsHistoryEntry]{version: 1, defaultLimit: DefaultRuleStatsHistoryLimit}

// RuleStatsHistoryPathForBase derives the rule-stats history file path from
// the scan cache path, mirroring the slop-history naming convention.
func RuleStatsHistoryPathForBase(base string) string {
	return derivedCachePath(base, ".rule-stats-history")
}

// LoadRuleStatsHistory reads the persisted per-scan rule stats, oldest first.
// A missing or unreadable file yields an empty history.
func LoadRuleStatsHistory(path string) []core.RuleStatsHistoryEntry {
	return ruleStatsHistory.Load(path)
}

// AppendRuleStatsHistory records one scan's rule stats, trims the history to
// limit entries, and persists the file, returning any write error.
func AppendRuleStatsHistory(path string, entry core.RuleStatsHistoryEntry, limit int) error {
	return ruleStatsHistory.Append(path, entry, limit)
}

// RecordRuleStatsHistory persists one scan's rule stats under the cache
// directory so later `doctor` runs can flag rules that teams mostly suppress.
// Patch scans (which run against materialized temp targets) and scans with the
// cache explicitly disabled are skipped. A failed write is returned as a
// warn-level operational diagnostic rather than dropped.
func RecordRuleStatsHistory(sc Context, rules []core.RuleStatsEntry) []core.Diagnostic {
	if len(rules) == 0 || strings.TrimSpace(sc.Opts.DiffText) != "" {
		return nil
	}
	if sc.Cfg.Cache.Enabled != nil && !*sc.Cfg.Cache.Enabled {
		return nil
	}
	path := RuleStatsHistoryPathForBase(sc.Cfg.Cache.Path)
	if path == "" {
		return nil
	}
	err := AppendRuleStatsHistory(path, core.RuleStatsHistoryEntry{
		Timestamp: sc.Today.UTC().Format(time.RFC3339),
		Rules:     rules,
	}, DefaultRuleStatsHistoryLimit)
	if err != nil {
		return []core.Diagnostic{HistoryWriteDiagnostic("rule-stats", path, err)}
	}
	return nil
}
