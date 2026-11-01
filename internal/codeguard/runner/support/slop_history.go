package support

import (
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// DefaultSlopHistoryLimit caps how many scans are retained per target key.
const DefaultSlopHistoryLimit = 100

var slopHistory = keyedHistory[core.SlopHistoryEntry]{version: 1, defaultLimit: DefaultSlopHistoryLimit}

// SlopHistoryPathForBase derives the slop-history file path from the scan
// cache path, mirroring the semantic cache naming convention.
func SlopHistoryPathForBase(base string) string {
	return derivedCachePath(base, ".slop-history")
}

// LoadSlopHistory reads the persisted slop-score history keyed by artifact
// ID. A missing or unreadable file yields an empty history.
func LoadSlopHistory(path string) map[string][]core.SlopHistoryEntry {
	return slopHistory.Load(path)
}

// AppendSlopHistory records a new observation for key, trims the history to
// limit entries, and persists the file. It returns the previous entry, if
// one existed, so callers can report score deltas, plus any write error.
func AppendSlopHistory(path string, key string, entry core.SlopHistoryEntry, limit int) (core.SlopHistoryEntry, bool, error) {
	return slopHistory.Append(path, key, entry, limit)
}
