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
	Frameworks   []FrameworkRef `json:"frameworks,omitempty"`
	Prompt       PromptTemplate `json:"prompt,omitempty"`
}

type requestHashPayload struct {
	Version      int            `json:"version"`
	Runtime      string         `json:"runtime"`
	TargetName   string         `json:"target_name"`
	Language     string         `json:"language"`
	BaseRef      string         `json:"base_ref,omitempty"`
	Diff         string         `json:"diff,omitempty"`
	ChangedFiles []string       `json:"changed_files,omitempty"`
	Checks       []CheckSpec    `json:"checks"`
	SourceFiles  []FileSnapshot `json:"source_files,omitempty"`
	TestFiles    []FileSnapshot `json:"test_files,omitempty"`
	Frameworks   []FrameworkRef `json:"frameworks,omitempty"`
	Prompt       PromptTemplate `json:"prompt,omitempty"`
}

func (req Request) hashPayload() requestHashPayload {
	return requestHashPayload{
		Version:      req.Version,
		Runtime:      req.Runtime,
		TargetName:   req.TargetName,
		Language:     req.Language,
		BaseRef:      req.BaseRef,
		Diff:         req.Diff,
		ChangedFiles: req.ChangedFiles,
		Checks:       req.Checks,
		SourceFiles:  req.SourceFiles,
		TestFiles:    req.TestFiles,
		Frameworks:   req.Frameworks,
		Prompt:       req.Prompt,
	}
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

type FrameworkRef struct {
	Name    string   `json:"name"`
	Path    string   `json:"path"`
	Signals []string `json:"signals,omitempty"`
	Hints   []string `json:"hints,omitempty"`
}

type PromptTemplate struct {
	Overview              string               `json:"overview,omitempty"`
	ResponseRequirements  []string             `json:"response_requirements,omitempty"`
	RuleInstructions      []RulePromptTemplate `json:"rule_instructions,omitempty"`
	FrameworkInstructions []FrameworkPromptRef `json:"framework_instructions,omitempty"`
}

type RulePromptTemplate struct {
	RuleID    string   `json:"rule_id"`
	Focus     string   `json:"focus"`
	Consider  []string `json:"consider,omitempty"`
	Avoid     []string `json:"avoid,omitempty"`
	Threshold string   `json:"threshold,omitempty"`
}

type FrameworkPromptRef struct {
	Name   string   `json:"name"`
	Path   string   `json:"path"`
	Hints  []string `json:"hints,omitempty"`
	Advice []string `json:"advice,omitempty"`
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
	ContractDrift           bool
	MisleadingErrorMessages bool
	TestBehaviorCoverage    bool
	TestAdequacy            bool
	// PerformanceReview adds the performance lens (performance.ai.semantic-perf)
	// to the request. It is driven by checks.performance being enabled, so that
	// requests are byte-identical to previous releases when the performance
	// section is off.
	PerformanceReview bool
}
