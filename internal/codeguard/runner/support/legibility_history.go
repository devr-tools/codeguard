package support

import (
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// DefaultLegibilityHistoryLimit caps how many scans are retained per target
// key, mirroring DefaultSlopHistoryLimit.
const DefaultLegibilityHistoryLimit = 100

var legibilityHistory = keyedHistory[core.LegibilityHistoryEntry]{version: 1, defaultLimit: DefaultLegibilityHistoryLimit}

// LegibilityHistoryPathForBase derives the repo_legibility history file path
// from the scan cache path, mirroring SlopHistoryPathForBase.
func LegibilityHistoryPathForBase(base string) string {
	return derivedCachePath(base, ".legibility-history")
}

// LoadLegibilityHistory reads the persisted legibility-score history keyed by
// artifact ID. A missing or unreadable file yields an empty history.
func LoadLegibilityHistory(path string) map[string][]core.LegibilityHistoryEntry {
	return legibilityHistory.Load(path)
}

// AppendLegibilityHistory records a new observation for key, trims the
// history to limit entries, and persists the file. It returns the previous
// entry, if one existed, so callers can report score deltas, plus any write
// error.
func AppendLegibilityHistory(path string, key string, entry core.LegibilityHistoryEntry, limit int) (core.LegibilityHistoryEntry, bool, error) {
	return legibilityHistory.Append(path, key, entry, limit)
}
