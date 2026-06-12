package nlrule

import "context"

const (
	runtimeCommandEnv = "CODEGUARD_AI_RUNTIME_COMMAND"
	maxSourceBytes    = 64 * 1024
)

type Runtime interface {
	Enabled() bool
	Fingerprint() string
	Evaluate(context.Context, EvaluationRequest) (EvaluationResponse, error)
}

type EvaluationRequest struct {
	Version string   `json:"version"`
	Rule    RuleSpec `json:"rule"`
	File    FileSpec `json:"file"`
	Prompt  string   `json:"prompt"`
}

type RuleSpec struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Message     string `json:"message"`
	Instruction string `json:"instruction"`
}

type FileSpec struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated,omitempty"`
}

type EvaluationResponse struct {
	Matches []Match `json:"matches"`
}

type Match struct {
	Line      int    `json:"line,omitempty"`
	Column    int    `json:"column,omitempty"`
	Message   string `json:"message,omitempty"`
	Rationale string `json:"rationale,omitempty"`
}
