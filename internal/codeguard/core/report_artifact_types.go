package core

type Artifact struct {
	ID              string                   `json:"id"`
	Kind            string                   `json:"kind"`
	Language        string                   `json:"language,omitempty"`
	Target          string                   `json:"target,omitempty"`
	DependencyGraph *DependencyGraphArtifact `json:"dependency_graph,omitempty"`
	SlopScore       *SlopScoreArtifact       `json:"slop_score,omitempty"`
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
