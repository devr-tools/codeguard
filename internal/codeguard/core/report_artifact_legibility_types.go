package core

// RepoLegibilityArtifact scores how legible a repository is to AI coding
// agents on a 0-100 scale (higher is better). The score aggregates agent-doc
// presence, doc/README drift, oversized-file ratio, basename ambiguity, and
// README presence; Components carries the per-signal breakdown so the score
// is explainable rather than a bare number.
type RepoLegibilityArtifact struct {
	Score         int                       `json:"score"`
	Components    []RepoLegibilityComponent `json:"components,omitempty"`
	PreviousScore *int                      `json:"previous_score,omitempty"`
	Delta         *int                      `json:"delta,omitempty"`
}

// RepoLegibilityComponent is one explainable slice of the legibility score:
// the points earned out of the component's maximum, plus a human-readable
// detail describing what was measured.
type RepoLegibilityComponent struct {
	Label  string `json:"label"`
	Score  int    `json:"score"`
	Max    int    `json:"max"`
	Detail string `json:"detail,omitempty"`
}
