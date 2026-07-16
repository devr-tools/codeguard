package core

type Artifact struct {
	ID               string                    `json:"id"`
	Kind             string                    `json:"kind"`
	Language         string                    `json:"language,omitempty"`
	Target           string                    `json:"target,omitempty"`
	DependencyGraph  *DependencyGraphArtifact  `json:"dependency_graph,omitempty"`
	SupplyChain      *SupplyChainArtifact      `json:"supply_chain,omitempty"`
	SlopScore        *SlopScoreArtifact        `json:"slop_score,omitempty"`
	PerformanceScore *PerformanceScoreArtifact `json:"performance_score,omitempty"`
	RuleStats        *RuleStatsArtifact        `json:"rule_stats,omitempty"`
	ChangeRisk       *ChangeRiskArtifact       `json:"change_risk,omitempty"`
	AIAnalysis       *AIAnalysisArtifact       `json:"ai_analysis,omitempty"`
	AIFix            *AIFixArtifact            `json:"ai_fix,omitempty"`
	ChangeImpact     *ChangeImpactArtifact     `json:"change_impact,omitempty"`
	RepoLegibility   *RepoLegibilityArtifact   `json:"repo_legibility,omitempty"`
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

type SupplyChainArtifact struct {
	Manifests []SupplyChainManifest `json:"manifests"`
}

type SupplyChainManifest struct {
	Ecosystem      string                  `json:"ecosystem"`
	Path           string                  `json:"path"`
	Name           string                  `json:"name,omitempty"`
	License        string                  `json:"license,omitempty"`
	LicenseLine    int                     `json:"license_line,omitempty"`
	PackageManager string                  `json:"package_manager,omitempty"`
	Lockfiles      []string                `json:"lockfiles,omitempty"`
	Dependencies   []SupplyChainDependency `json:"dependencies,omitempty"`
	// AnalysisLimitations records dependency declarations that were intentionally
	// not evaluated because resolving them would require executing project code.
	AnalysisLimitations []string `json:"analysis_limitations,omitempty"`
}

type SupplyChainDependency struct {
	Name              string                        `json:"name"`
	Requirement       string                        `json:"requirement,omitempty"`
	Version           string                        `json:"version,omitempty"`
	Scope             string                        `json:"scope,omitempty"`
	Groups            []string                      `json:"groups,omitempty"`
	Indirect          bool                          `json:"indirect,omitempty"`
	Pinned            bool                          `json:"pinned,omitempty"`
	Line              int                           `json:"line,omitempty"`
	License           string                        `json:"license,omitempty"`
	LicenseSource     string                        `json:"license_source,omitempty"`
	LicenseCandidates []SupplyChainLicenseCandidate `json:"license_candidates,omitempty"`
}

type SupplyChainLicenseCandidate struct {
	License    string `json:"license"`
	Confidence string `json:"confidence,omitempty"`
	Provenance string `json:"provenance,omitempty"`
	Source     string `json:"source,omitempty"`
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
