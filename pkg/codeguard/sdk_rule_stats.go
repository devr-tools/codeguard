package codeguard

import "github.com/devr-tools/codeguard/internal/codeguard/runner"

// RuleStatsHistoryPath derives the rule-stats history file path for a config.
func RuleStatsHistoryPath(cfg Config) string {
	return runner.RuleStatsHistoryPath(cfg)
}

// LoadRuleStatsHistory reads the persisted per-scan rule suppression stats,
// oldest first.
func LoadRuleStatsHistory(path string) []RuleStatsHistoryEntry {
	return runner.LoadRuleStatsHistory(path)
}
