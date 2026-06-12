package core

type Artifact struct {
	ID              string                   `json:"id"`
	Kind            string                   `json:"kind"`
	Language        string                   `json:"language,omitempty"`
	Target          string                   `json:"target,omitempty"`
	DependencyGraph *DependencyGraphArtifact `json:"dependency_graph,omitempty"`
	SlopScore       *SlopScoreArtifact       `json:"slop_score,omitempty"`
	AIAnalysis      *AIAnalysisArtifact      `json:"ai_analysis,omitempty"`
	AIFix           *AIFixArtifact           `json:"ai_fix,omitempty"`
	ChangeImpact    *ChangeImpactArtifact    `json:"change_impact,omitempty"`
}

type DependencyGraphArtifact struct {
	Order []string              `json:"order,omitempty"`
	Nodes []DependencyGraphNode `json:"nodes"`
}

type DependencyGraphNode struct {
	ID       string                `json:"id"`
	Path     string                `json:"path,omitempty"`
	IsPublic bool                  `json:"is_public,omitempty"`
	Edges    []DependencyGraphEdge `json:"edges,omitempty"`
}

type DependencyGraphEdge struct {
	To    string   `json:"to"`
	Line  int      `json:"line,omitempty"`
	Names []string `json:"names,omitempty"`
}

type SlopScoreArtifact struct {
	Score         int                  `json:"score"`
	Signals       int                  `json:"signals"`
	Components    []SlopScoreComponent `json:"components,omitempty"`
	PreviousScore *int                 `json:"previous_score,omitempty"`
	Delta         *int                 `json:"delta,omitempty"`
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

const ReportArtifactKindChangeImpact = "change-impact"

// ChangeImpactArtifact summarizes the transitive dependency impact of changed
// modules in diff mode.
type ChangeImpactArtifact struct {
	BaseRef string              `json:"base_ref,omitempty"`
	Entries []ChangeImpactEntry `json:"entries"`
}

// ChangeImpactEntry records the impact radius of one changed module.
type ChangeImpactEntry struct {
	Target               string   `json:"target"`
	Language             string   `json:"language"`
	Module               string   `json:"module"`
	File                 string   `json:"file"`
	TransitiveDependents int      `json:"transitive_dependents"`
	Dependents           []string `json:"dependents,omitempty"`
}

func NewChangeImpactArtifact(baseRef string, entries []ChangeImpactEntry) Artifact {
	return Artifact{
		ID:   "change_impact",
		Kind: ReportArtifactKindChangeImpact,
		ChangeImpact: &ChangeImpactArtifact{
			BaseRef: baseRef,
			Entries: entries,
		},
	}
}
