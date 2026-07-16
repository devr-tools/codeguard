package core

// Performance-score artifact and history types, mirroring the slop_score
// shapes so tooling can trend both metrics the same way. Components reuse
// SlopScoreComponent: its fields (rule_id/count/weight/contribution) are
// score-agnostic.

// PerformanceScoreArtifact is the per-target performance score published by
// the performance section: a weighted count of its findings by rule family,
// saturating at 100.
type PerformanceScoreArtifact struct {
	Score         int                  `json:"score"`
	Signals       int                  `json:"signals"`
	Components    []SlopScoreComponent `json:"components,omitempty"`
	PreviousScore *int                 `json:"previous_score,omitempty"`
	Delta         *int                 `json:"delta,omitempty"`
}

// PerformanceHistoryEntry is one persisted performance-score observation for
// a target, recorded once per scan so trends can be reported over time.
type PerformanceHistoryEntry struct {
	Timestamp  string               `json:"timestamp"`
	Score      int                  `json:"score"`
	Signals    int                  `json:"signals"`
	Components []SlopScoreComponent `json:"components,omitempty"`
}
