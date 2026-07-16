package core

// LegibilityHistoryEntry is one persisted repo_legibility observation for a
// target, recorded once per scan so the AI-readiness trend can be reported
// over time, mirroring SlopHistoryEntry and PerformanceHistoryEntry.
type LegibilityHistoryEntry struct {
	Timestamp  string                    `json:"timestamp"`
	Score      int                       `json:"score"`
	Components []RepoLegibilityComponent `json:"components,omitempty"`
}
