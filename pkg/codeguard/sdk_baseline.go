package codeguard

import "github.com/devr-tools/codeguard/internal/codeguard/runner"

func WriteBaselineFile(path string, entries []BaselineEntry) error {
	return runner.WriteBaselineFile(path, entries)
}

// SlopHistoryPath derives the slop-score history file path for a config.
func SlopHistoryPath(cfg Config) string {
	return runner.SlopHistoryPath(cfg)
}

// LoadSlopHistory reads the persisted slop-score trend, keyed by artifact ID.
func LoadSlopHistory(path string) map[string][]SlopHistoryEntry {
	return runner.LoadSlopHistory(path)
}

func BaselineEntriesFromReport(rep Report) []BaselineEntry {
	return runner.BaselineEntriesFromReport(rep)
}
