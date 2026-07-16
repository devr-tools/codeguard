package runner

import (
	"github.com/devr-tools/codeguard/internal/codeguard/config"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

func WriteBaselineFile(path string, entries []core.BaselineEntry) error {
	return runnersupport.WriteBaselineFile(path, entries)
}

func SlopHistoryPath(cfg core.Config) string {
	config.ApplyDefaults(&cfg)
	return runnersupport.SlopHistoryPathForBase(cfg.Cache.Path)
}

func LoadSlopHistory(path string) map[string][]core.SlopHistoryEntry {
	return runnersupport.LoadSlopHistory(path)
}

func PerfScoreHistoryPath(cfg core.Config) string {
	config.ApplyDefaults(&cfg)
	return runnersupport.PerfScoreHistoryPathForBase(cfg.Cache.Path)
}

func LoadPerfScoreHistory(path string) map[string][]core.PerformanceHistoryEntry {
	return runnersupport.LoadPerfScoreHistory(path)
}

func RuleStatsHistoryPath(cfg core.Config) string {
	config.ApplyDefaults(&cfg)
	return runnersupport.RuleStatsHistoryPathForBase(cfg.Cache.Path)
}

func LoadRuleStatsHistory(path string) []core.RuleStatsHistoryEntry {
	return runnersupport.LoadRuleStatsHistory(path)
}

func BaselineEntriesFromReport(report core.Report) []core.BaselineEntry {
	return runnersupport.BaselineEntriesFromReport(report)
}
