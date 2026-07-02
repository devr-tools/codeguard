package core

const ReportArtifactKindRuleStats = "rule_stats"

// RuleStatsArtifact summarizes per-rule finding activity for one scan: how many
// findings each rule emitted and how many were silenced by the baseline, a
// waiver, or an inline codeguard:ignore directive. Rules with no activity are
// omitted.
type RuleStatsArtifact struct {
	Rules []RuleStatsEntry `json:"rules"`
}

// RuleStatsEntry records one rule's emission and suppression counts.
// SuppressionRatio is suppressed/(emitted+suppressed); a persistently high
// ratio signals a rule teams work around rather than act on.
type RuleStatsEntry struct {
	RuleID             string  `json:"rule_id"`
	Emitted            int     `json:"emitted"`
	BaselineSuppressed int     `json:"baseline_suppressed"`
	WaiverSuppressed   int     `json:"waiver_suppressed"`
	InlineSuppressed   int     `json:"inline_suppressed"`
	SuppressionRatio   float64 `json:"suppression_ratio"`
}

// Suppressed returns the total findings silenced for the rule across all
// suppression mechanisms.
func (entry RuleStatsEntry) Suppressed() int {
	return entry.BaselineSuppressed + entry.WaiverSuppressed + entry.InlineSuppressed
}

// RuleStatsHistoryEntry is one persisted per-scan rule-stats observation,
// recorded once per scan so rule health can be inspected after the fact.
type RuleStatsHistoryEntry struct {
	Timestamp string           `json:"timestamp"`
	Rules     []RuleStatsEntry `json:"rules"`
}

func NewRuleStatsArtifact(rules []RuleStatsEntry) Artifact {
	return Artifact{
		ID:   "rule_stats",
		Kind: ReportArtifactKindRuleStats,
		RuleStats: &RuleStatsArtifact{
			Rules: rules,
		},
	}
}
