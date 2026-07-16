package support

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

const perfScoreHistoryVersion = 1

// DefaultPerfScoreHistoryLimit caps how many scans are retained per target
// key, mirroring DefaultSlopHistoryLimit.
const DefaultPerfScoreHistoryLimit = 100

type perfScoreHistoryFile struct {
	Version int                                       `json:"version"`
	Entries map[string][]core.PerformanceHistoryEntry `json:"entries"`
}

// PerfScoreHistoryPathForBase derives the performance-score history file path
// from the scan cache path, mirroring SlopHistoryPathForBase.
func PerfScoreHistoryPathForBase(base string) string {
	return derivedCachePath(base, ".perf-history")
}

// LoadPerfScoreHistory reads the persisted performance-score history keyed by
// artifact ID. A missing or unreadable file yields an empty history.
func LoadPerfScoreHistory(path string) map[string][]core.PerformanceHistoryEntry {
	if strings.TrimSpace(path) == "" {
		return map[string][]core.PerformanceHistoryEntry{}
	}
	data, err := os.ReadFile(path) //nolint:gosec // config-supplied perf-history cache path
	if err != nil {
		return map[string][]core.PerformanceHistoryEntry{}
	}
	var file perfScoreHistoryFile
	if err := json.Unmarshal(data, &file); err != nil || file.Version != perfScoreHistoryVersion || file.Entries == nil {
		return map[string][]core.PerformanceHistoryEntry{}
	}
	return file.Entries
}

// AppendPerfScoreHistory records a new observation for key, trims the history
// to limit entries, and persists the file. It returns the previous entry, if
// one existed, so callers can report score deltas.
func AppendPerfScoreHistory(path string, key string, entry core.PerformanceHistoryEntry, limit int) (core.PerformanceHistoryEntry, bool) {
	if strings.TrimSpace(path) == "" || strings.TrimSpace(key) == "" {
		return core.PerformanceHistoryEntry{}, false
	}
	if limit <= 0 {
		limit = DefaultPerfScoreHistoryLimit
	}
	entries := LoadPerfScoreHistory(path)
	history := entries[key]
	var previous core.PerformanceHistoryEntry
	hasPrevious := len(history) > 0
	if hasPrevious {
		previous = history[len(history)-1]
	}
	history = append(history, entry)
	if len(history) > limit {
		history = history[len(history)-limit:]
	}
	entries[key] = history
	savePerfScoreHistory(path, entries)
	return previous, hasPrevious
}

func savePerfScoreHistory(path string, entries map[string][]core.PerformanceHistoryEntry) {
	payload := perfScoreHistoryFile{Version: perfScoreHistoryVersion, Entries: entries}
	writeHistoryFile(path, payload)
}
