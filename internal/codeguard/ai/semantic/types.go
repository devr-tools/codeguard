package semantic

type Request struct {
	Version      int            `json:"version"`
	Runtime      string         `json:"runtime"`
	TargetName   string         `json:"target_name"`
	TargetPath   string         `json:"target_path"`
	Language     string         `json:"language"`
	BaseRef      string         `json:"base_ref,omitempty"`
	Diff         string         `json:"diff,omitempty"`
	ChangedFiles []string       `json:"changed_files,omitempty"`
	Checks       []CheckSpec    `json:"checks"`
	SourceFiles  []FileSnapshot `json:"source_files,omitempty"`
	TestFiles    []FileSnapshot `json:"test_files,omitempty"`
}

type CheckSpec struct {
	RuleID      string `json:"rule_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type FileSnapshot struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type Response struct {
	Verdicts []Verdict `json:"verdicts"`
}

type Verdict struct {
	RuleID     string `json:"rule_id"`
	Path       string `json:"path"`
	Line       int    `json:"line,omitempty"`
	Level      string `json:"level,omitempty"`
	Message    string `json:"message"`
	Confidence string `json:"confidence,omitempty"`
}

type CheckSelection struct {
	FunctionContract        bool
	MisleadingErrorMessages bool
	TestBehaviorCoverage    bool
}
