package support

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

const legibilityHistoryVersion = 1

// DefaultLegibilityHistoryLimit caps how many scans are retained per target
// key, mirroring DefaultSlopHistoryLimit.
const DefaultLegibilityHistoryLimit = 100

type legibilityHistoryFile struct {
	Version int                                      `json:"version"`
	Entries map[string][]core.LegibilityHistoryEntry `json:"entries"`
}

// LegibilityHistoryPathForBase derives the repo_legibility history file path
// from the scan cache path, mirroring SlopHistoryPathForBase.
func LegibilityHistoryPathForBase(base string) string {
	trimmed := strings.TrimSpace(base)
	if trimmed == "" {
		return ""
	}
	ext := filepath.Ext(trimmed)
	if ext == "" {
		return trimmed + ".legibility-history"
	}
	return strings.TrimSuffix(trimmed, ext) + ".legibility-history" + ext
}

// LoadLegibilityHistory reads the persisted legibility-score history keyed by
// artifact ID. A missing or unreadable file yields an empty history.
func LoadLegibilityHistory(path string) map[string][]core.LegibilityHistoryEntry {
	if strings.TrimSpace(path) == "" {
		return map[string][]core.LegibilityHistoryEntry{}
	}
	data, err := os.ReadFile(path) //nolint:gosec // config-supplied legibility-history cache path
	if err != nil {
		return map[string][]core.LegibilityHistoryEntry{}
	}
	var file legibilityHistoryFile
	if err := json.Unmarshal(data, &file); err != nil || file.Version != legibilityHistoryVersion || file.Entries == nil {
		return map[string][]core.LegibilityHistoryEntry{}
	}
	return file.Entries
}

// AppendLegibilityHistory records a new observation for key, trims the
// history to limit entries, and persists the file. It returns the previous
// entry, if one existed, so callers can report score deltas.
func AppendLegibilityHistory(path string, key string, entry core.LegibilityHistoryEntry, limit int) (core.LegibilityHistoryEntry, bool) {
	if strings.TrimSpace(path) == "" || strings.TrimSpace(key) == "" {
		return core.LegibilityHistoryEntry{}, false
	}
	if limit <= 0 {
		limit = DefaultLegibilityHistoryLimit
	}
	entries := LoadLegibilityHistory(path)
	history := entries[key]
	var previous core.LegibilityHistoryEntry
	hasPrevious := len(history) > 0
	if hasPrevious {
		previous = history[len(history)-1]
	}
	history = append(history, entry)
	if len(history) > limit {
		history = history[len(history)-limit:]
	}
	entries[key] = history
	saveLegibilityHistory(path, entries)
	return previous, hasPrevious
}

func saveLegibilityHistory(path string, entries map[string][]core.LegibilityHistoryEntry) {
	payload := legibilityHistoryFile{Version: legibilityHistoryVersion, Entries: entries}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return
	}
	_ = os.WriteFile(path, append(data, '\n'), 0o600)
}
