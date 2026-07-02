package support

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

const ruleStatsHistoryVersion = 1

// DefaultRuleStatsHistoryLimit caps how many scans of rule stats are retained,
// mirroring the slop-history cap.
const DefaultRuleStatsHistoryLimit = 100

type ruleStatsHistoryFile struct {
	Version int                          `json:"version"`
	Entries []core.RuleStatsHistoryEntry `json:"entries"`
}

// RuleStatsHistoryPathForBase derives the rule-stats history file path from
// the scan cache path, mirroring the slop-history naming convention.
func RuleStatsHistoryPathForBase(base string) string {
	trimmed := strings.TrimSpace(base)
	if trimmed == "" {
		return ""
	}
	ext := filepath.Ext(trimmed)
	if ext == "" {
		return trimmed + ".rule-stats-history"
	}
	return strings.TrimSuffix(trimmed, ext) + ".rule-stats-history" + ext
}

// LoadRuleStatsHistory reads the persisted per-scan rule stats, oldest first.
// A missing or unreadable file yields an empty history.
func LoadRuleStatsHistory(path string) []core.RuleStatsHistoryEntry {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	data, err := os.ReadFile(path) //nolint:gosec // config-supplied rule-stats history cache path
	if err != nil {
		return nil
	}
	var file ruleStatsHistoryFile
	if err := json.Unmarshal(data, &file); err != nil || file.Version != ruleStatsHistoryVersion {
		return nil
	}
	return file.Entries
}

// AppendRuleStatsHistory records one scan's rule stats, trims the history to
// limit entries, and persists the file.
func AppendRuleStatsHistory(path string, entry core.RuleStatsHistoryEntry, limit int) {
	if strings.TrimSpace(path) == "" {
		return
	}
	if limit <= 0 {
		limit = DefaultRuleStatsHistoryLimit
	}
	entries := append(LoadRuleStatsHistory(path), entry)
	if len(entries) > limit {
		entries = entries[len(entries)-limit:]
	}
	saveRuleStatsHistory(path, entries)
}

// RecordRuleStatsHistory persists one scan's rule stats under the cache
// directory so later `doctor` runs can flag rules that teams mostly suppress.
// Patch scans (which run against materialized temp targets) and scans with the
// cache explicitly disabled are skipped.
func RecordRuleStatsHistory(sc Context, rules []core.RuleStatsEntry) {
	if len(rules) == 0 || strings.TrimSpace(sc.Opts.DiffText) != "" {
		return
	}
	if sc.Cfg.Cache.Enabled != nil && !*sc.Cfg.Cache.Enabled {
		return
	}
	path := RuleStatsHistoryPathForBase(sc.Cfg.Cache.Path)
	if path == "" {
		return
	}
	AppendRuleStatsHistory(path, core.RuleStatsHistoryEntry{
		Timestamp: sc.Today.UTC().Format(time.RFC3339),
		Rules:     rules,
	}, DefaultRuleStatsHistoryLimit)
}

func saveRuleStatsHistory(path string, entries []core.RuleStatsHistoryEntry) {
	payload := ruleStatsHistoryFile{Version: ruleStatsHistoryVersion, Entries: entries}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return
	}
	_ = os.WriteFile(path, append(data, '\n'), 0o600)
}
