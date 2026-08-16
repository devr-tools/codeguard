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

// WaiverAuditHistoryPath derives the waiver-audit history file path for a config.
func WaiverAuditHistoryPath(cfg Config) string {
	return runner.WaiverAuditHistoryPath(cfg)
}

// LoadWaiverAuditHistory reads persisted waiver-audit observations, oldest first.
func LoadWaiverAuditHistory(path string) []WaiverAuditHistoryEntry {
	return runner.LoadWaiverAuditHistory(path)
}
