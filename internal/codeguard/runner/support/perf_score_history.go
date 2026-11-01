package support

import (
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// DefaultPerfScoreHistoryLimit caps how many scans are retained per target
// key, mirroring DefaultSlopHistoryLimit.
const DefaultPerfScoreHistoryLimit = 100

var perfScoreHistory = keyedHistory[core.PerformanceHistoryEntry]{version: 1, defaultLimit: DefaultPerfScoreHistoryLimit}

// PerfScoreHistoryPathForBase derives the performance-score history file path
// from the scan cache path, mirroring SlopHistoryPathForBase.
func PerfScoreHistoryPathForBase(base string) string {
	return derivedCachePath(base, ".perf-history")
}

// LoadPerfScoreHistory reads the persisted performance-score history keyed by
// artifact ID. A missing or unreadable file yields an empty history.
func LoadPerfScoreHistory(path string) map[string][]core.PerformanceHistoryEntry {
	return perfScoreHistory.Load(path)
}

// AppendPerfScoreHistory records a new observation for key, trims the history
// to limit entries, and persists the file. It returns the previous entry, if
// one existed, so callers can report score deltas, plus any write error.
func AppendPerfScoreHistory(path string, key string, entry core.PerformanceHistoryEntry, limit int) (core.PerformanceHistoryEntry, bool, error) {
	return perfScoreHistory.Append(path, key, entry, limit)
}
