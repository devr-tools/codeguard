package core

// Slop-score and AI analysis artifact types reported alongside findings.

type SlopScoreArtifact struct {
	Score         int                  `json:"score"`
	Signals       int                  `json:"signals"`
	Components    []SlopScoreComponent `json:"components,omitempty"`
	PreviousScore *int                 `json:"previous_score,omitempty"`
	Delta         *int                 `json:"delta,omitempty"`
}

type ChangeRiskArtifact struct {
	Score                int                   `json:"score"`
	Level                string                `json:"level,omitempty"`
	ProvenanceActive     bool                  `json:"provenance_active,omitempty"`
	ChangedFiles         int                   `json:"changed_files,omitempty"`
	HighImpactChange     bool                  `json:"high_impact_change,omitempty"`
	AIFindingCount       int                   `json:"ai_finding_count,omitempty"`
	SemanticFindingCount int                   `json:"semantic_finding_count,omitempty"`
	Components           []ChangeRiskComponent `json:"components,omitempty"`
}

type ChangeRiskComponent struct {
	Label        string `json:"label"`
	Contribution int    `json:"contribution"`
	Detail       string `json:"detail,omitempty"`
}

// SlopHistoryEntry is one persisted slop-score observation for a target,
// recorded once per scan so trends can be reported over time.
type SlopHistoryEntry struct {
	Timestamp  string               `json:"timestamp"`
	Score      int                  `json:"score"`
	Signals    int                  `json:"signals"`
	Components []SlopScoreComponent `json:"components,omitempty"`
}

type SlopScoreComponent struct {
	RuleID       string `json:"rule_id"`
	Count        int    `json:"count"`
	Weight       int    `json:"weight"`
	Contribution int    `json:"contribution"`
}

type AIAnalysisArtifact struct {
	Provider string              `json:"provider,omitempty"`
	Mode     string              `json:"mode,omitempty"`
	Verdicts []AIAnalysisVerdict `json:"verdicts,omitempty"`
}

type AIAnalysisVerdict struct {
	ID          string `json:"id,omitempty"`
	Kind        string `json:"kind,omitempty"`
	RuleID      string `json:"rule_id,omitempty"`
	Path        string `json:"path,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	ContentHash string `json:"content_hash,omitempty"`
	Status      string `json:"status,omitempty"`
	Summary     string `json:"summary,omitempty"`
}

type AIFixArtifact struct {
	RuleID    string   `json:"rule_id,omitempty"`
	Path      string   `json:"path,omitempty"`
	Verified  bool     `json:"verified,omitempty"`
	Patch     string   `json:"patch,omitempty"`
	ChecksRun []string `json:"checks_run,omitempty"`
	TestsRun  []string `json:"tests_run,omitempty"`
	Summary   string   `json:"summary,omitempty"`
}
