package support

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

const slopHistoryVersion = 1

// DefaultSlopHistoryLimit caps how many scans are retained per target key.
const DefaultSlopHistoryLimit = 100

type slopHistoryFile struct {
	Version int                                `json:"version"`
	Entries map[string][]core.SlopHistoryEntry `json:"entries"`
}

// SlopHistoryPathForBase derives the slop-history file path from the scan
// cache path, mirroring the semantic cache naming convention.
func SlopHistoryPathForBase(base string) string {
	trimmed := strings.TrimSpace(base)
	if trimmed == "" {
		return ""
	}
	ext := filepath.Ext(trimmed)
	if ext == "" {
		return trimmed + ".slop-history"
	}
	return strings.TrimSuffix(trimmed, ext) + ".slop-history" + ext
}

// LoadSlopHistory reads the persisted slop-score history keyed by artifact
// ID. A missing or unreadable file yields an empty history.
func LoadSlopHistory(path string) map[string][]core.SlopHistoryEntry {
	if strings.TrimSpace(path) == "" {
		return map[string][]core.SlopHistoryEntry{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string][]core.SlopHistoryEntry{}
	}
	var file slopHistoryFile
	if err := json.Unmarshal(data, &file); err != nil || file.Version != slopHistoryVersion || file.Entries == nil {
		return map[string][]core.SlopHistoryEntry{}
	}
	return file.Entries
}

// AppendSlopHistory records a new observation for key, trims the history to
// limit entries, and persists the file. It returns the previous entry, if
// one existed, so callers can report score deltas.
func AppendSlopHistory(path string, key string, entry core.SlopHistoryEntry, limit int) (core.SlopHistoryEntry, bool) {
	if strings.TrimSpace(path) == "" || strings.TrimSpace(key) == "" {
		return core.SlopHistoryEntry{}, false
	}
	if limit <= 0 {
		limit = DefaultSlopHistoryLimit
	}
	entries := LoadSlopHistory(path)
	history := entries[key]
	var previous core.SlopHistoryEntry
	hasPrevious := len(history) > 0
	if hasPrevious {
		previous = history[len(history)-1]
	}
	history = append(history, entry)
	if len(history) > limit {
		history = history[len(history)-limit:]
	}
	entries[key] = history
	saveSlopHistory(path, entries)
	return previous, hasPrevious
}

func saveSlopHistory(path string, entries map[string][]core.SlopHistoryEntry) {
	payload := slopHistoryFile{Version: slopHistoryVersion, Entries: entries}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(path, append(data, '\n'), 0o644)
}
