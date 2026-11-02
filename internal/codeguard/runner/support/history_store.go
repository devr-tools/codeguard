package support

import (
	"fmt"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/cachefile"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// HistoryWriteDiagnosticID identifies a scan whose trend history could not be
// persisted. The scan itself is complete; only the history file is stale.
const HistoryWriteDiagnosticID = "scan.history-write"

// keyedHistoryFile is the on-disk envelope for per-key trend histories
// (slop, performance score, legibility). Field order and tags are part of the
// file format and must stay stable so existing history files keep loading.
type keyedHistoryFile[T any] struct {
	Version int            `json:"version"`
	Entries map[string][]T `json:"entries"`
}

// listHistoryFile is the on-disk envelope for per-scan list histories (rule
// stats, waiver audit).
type listHistoryFile[T any] struct {
	Version int `json:"version"`
	Entries []T `json:"entries"`
}

// keyedHistory persists a bounded, per-key series of observations in a
// versioned JSON file.
type keyedHistory[T any] struct {
	version      int
	defaultLimit int
}

// Load reads the history keyed by artifact ID. A blank path, missing,
// malformed, or version-mismatched file yields an empty, non-nil map.
func (h keyedHistory[T]) Load(path string) map[string][]T {
	if strings.TrimSpace(path) == "" {
		return map[string][]T{}
	}
	var file keyedHistoryFile[T]
	if !cachefile.Load(path, &file) || file.Version != h.version || file.Entries == nil {
		return map[string][]T{}
	}
	return file.Entries
}

// Append records entry for key, trims that key's series to limit entries
// (defaultLimit when limit <= 0), and saves the file. It returns the previous
// entry for key, if one existed, and any error from persisting the file.
func (h keyedHistory[T]) Append(path string, key string, entry T, limit int) (T, bool, error) {
	var previous T
	if strings.TrimSpace(path) == "" || strings.TrimSpace(key) == "" {
		return previous, false, nil
	}
	if limit <= 0 {
		limit = h.defaultLimit
	}
	entries := h.Load(path)
	history := entries[key]
	hasPrevious := len(history) > 0
	if hasPrevious {
		previous = history[len(history)-1]
	}
	history = append(history, entry)
	if len(history) > limit {
		history = history[len(history)-limit:]
	}
	entries[key] = history
	return previous, hasPrevious, h.Save(path, entries)
}

// Save writes entries inside the versioned envelope.
func (h keyedHistory[T]) Save(path string, entries map[string][]T) error {
	return cachefile.Write(path, keyedHistoryFile[T]{Version: h.version, Entries: entries})
}

// listHistory persists a bounded, oldest-first list of per-scan observations
// in a versioned JSON file.
type listHistory[T any] struct {
	version      int
	defaultLimit int
}

// Load reads the history, oldest first. A blank path, missing, malformed, or
// version-mismatched file yields nil.
func (h listHistory[T]) Load(path string) []T {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	var file listHistoryFile[T]
	if !cachefile.Load(path, &file) || file.Version != h.version {
		return nil
	}
	return file.Entries
}

// Append records entry, trims the list to limit entries (defaultLimit when
// limit <= 0), and saves the file, returning any error from persisting it.
func (h listHistory[T]) Append(path string, entry T, limit int) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if limit <= 0 {
		limit = h.defaultLimit
	}
	entries := append(h.Load(path), entry)
	if len(entries) > limit {
		entries = entries[len(entries)-limit:]
	}
	return h.Save(path, entries)
}

// Save writes entries inside the versioned envelope.
func (h listHistory[T]) Save(path string, entries []T) error {
	return cachefile.Write(path, listHistoryFile[T]{Version: h.version, Entries: entries})
}

// HistoryWriteDiagnostic reports a trend-history file that could not be
// written. It warns rather than fails: the scan results are complete, but the
// named history will be missing this scan's observation.
func HistoryWriteDiagnostic(history string, path string, err error) core.Diagnostic {
	return core.Diagnostic{
		ID:          HistoryWriteDiagnosticID,
		Level:       "warn",
		Kind:        "infrastructure",
		Message:     fmt.Sprintf("could not write %s history: %v; this scan was not recorded in the trend", history, err),
		Path:        path,
		Operational: true,
		Metadata: map[string]string{
			"history":           history,
			"degradation":       "history-not-persisted",
			"findings_affected": "no",
		},
	}
}
