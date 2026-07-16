package support

import "github.com/devr-tools/codeguard/internal/codeguard/core"

type WarnFindingInput struct {
	RuleID     string
	Path       string
	Line       int
	Column     int
	Message    string
	Confidence string
}

func NewWarnFinding(env Context, input WarnFindingInput) core.Finding {
	return env.NewFinding(FindingInput{
		RuleID:     input.RuleID,
		Level:      "warn",
		Path:       input.Path,
		Line:       input.Line,
		Column:     input.Column,
		Message:    input.Message,
		Confidence: input.Confidence,
	})
}
