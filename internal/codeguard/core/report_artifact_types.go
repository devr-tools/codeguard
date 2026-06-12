package core

const ReportArtifactKindChangeImpact = "change-impact"

// ReportArtifact is a typed side output attached to a report alongside findings.
type ReportArtifact struct {
	Kind         string                `json:"kind"`
	ChangeImpact *ChangeImpactArtifact `json:"change_impact,omitempty"`
}

// ChangeImpactArtifact summarizes the transitive dependency impact of changed modules in diff mode.
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

func NewChangeImpactArtifact(baseRef string, entries []ChangeImpactEntry) ReportArtifact {
	return ReportArtifact{
		Kind: ReportArtifactKindChangeImpact,
		ChangeImpact: &ChangeImpactArtifact{
			BaseRef: baseRef,
			Entries: entries,
		},
	}
}
