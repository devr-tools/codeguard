package codeguard

import (
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/internal/codeguard/runner"
)

// LegibilityHistoryEntry is one persisted repo_legibility observation for a
// target.
type LegibilityHistoryEntry = core.LegibilityHistoryEntry

// LegibilityHistoryPath derives the repo_legibility history file path for a
// config, mirroring SlopHistoryPath and PerfScoreHistoryPath.
func LegibilityHistoryPath(cfg Config) string {
	return runner.LegibilityHistoryPath(cfg)
}

// LoadLegibilityHistory reads the persisted repo_legibility trend, keyed by
// artifact ID.
func LoadLegibilityHistory(path string) map[string][]LegibilityHistoryEntry {
	return runner.LoadLegibilityHistory(path)
}
