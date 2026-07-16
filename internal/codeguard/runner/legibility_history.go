package runner

import (
	"github.com/devr-tools/codeguard/internal/codeguard/config"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

// LegibilityHistoryPath derives the repo_legibility history file path for a
// config, mirroring SlopHistoryPath and PerfScoreHistoryPath.
func LegibilityHistoryPath(cfg core.Config) string {
	config.ApplyDefaults(&cfg)
	return runnersupport.LegibilityHistoryPathForBase(cfg.Cache.Path)
}

// LoadLegibilityHistory reads the persisted repo_legibility trend, keyed by
// artifact ID.
func LoadLegibilityHistory(path string) map[string][]core.LegibilityHistoryEntry {
	return runnersupport.LoadLegibilityHistory(path)
}
