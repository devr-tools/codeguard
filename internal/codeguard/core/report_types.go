package core

type BaselineFile struct {
	GeneratedAt string          `json:"generated_at"`
	Entries     []BaselineEntry `json:"entries"`
}

type BaselineEntry struct {
	Fingerprint string `json:"fingerprint"`
	RuleID      string `json:"rule_id,omitempty"`
	Path        string `json:"path,omitempty"`
	Message     string `json:"message,omitempty"`
}

type Status string

const (
	StatusPass Status = "pass"
	StatusWarn Status = "warn"
	StatusFail Status = "fail"
)

type Report struct {
	Name        string          `json:"name"`
	Profile     string          `json:"profile,omitempty"`
	GeneratedAt string          `json:"generated_at"`
	Sections    []SectionResult `json:"sections"`
	Artifacts   []Artifact      `json:"artifacts,omitempty"`
	Summary     ReportSummary   `json:"summary"`
}

type Artifact struct {
	ID              string                   `json:"id"`
	Kind            string                   `json:"kind"`
	Language        string                   `json:"language,omitempty"`
	Target          string                   `json:"target,omitempty"`
	DependencyGraph *DependencyGraphArtifact `json:"dependency_graph,omitempty"`
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

type SectionResult struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Status          Status    `json:"status"`
	Findings        []Finding `json:"findings"`
	SuppressedCount int       `json:"suppressed_count,omitempty"`
}

type Finding struct {
	RuleID            string `json:"rule_id"`
	Level             string `json:"level"`
	Severity          string `json:"severity,omitempty"`
	Title             string `json:"title,omitempty"`
	Section           string `json:"section,omitempty"`
	Message           string `json:"message"`
	Why               string `json:"why,omitempty"`
	HowToFix          string `json:"how_to_fix,omitempty"`
	Path              string `json:"path,omitempty"`
	Line              int    `json:"line,omitempty"`
	Column            int    `json:"column,omitempty"`
	Fingerprint       string `json:"fingerprint"`
	Suppressed        bool   `json:"suppressed,omitempty"`
	SuppressionReason string `json:"suppression_reason,omitempty"`
}

type ReportSummary struct {
	PassedSections     int `json:"passed_sections"`
	WarnedSections     int `json:"warned_sections"`
	FailedSections     int `json:"failed_sections"`
	TotalFindings      int `json:"total_findings"`
	SuppressedFindings int `json:"suppressed_findings"`
}
