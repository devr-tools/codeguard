package nlrule

import (
	"context"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type EvaluatedFinding struct {
	Line    int
	Column  int
	Message string
	Why     string
}

func EvaluateFile(ctx context.Context, runtime Runtime, rule core.CustomRuleConfig, path string, data []byte) ([]EvaluatedFinding, error) {
	if runtime == nil || !runtime.Enabled() || strings.TrimSpace(rule.NaturalLanguage) == "" {
		return nil, nil
	}
	response, err := runtime.Evaluate(ctx, Compile(rule, path, data))
	if err != nil {
		return nil, err
	}
	findings := make([]EvaluatedFinding, 0, len(response.Matches))
	for _, match := range response.Matches {
		message := strings.TrimSpace(match.Message)
		if message == "" {
			message = rule.Message
		}
		why := strings.TrimSpace(match.Rationale)
		if why == "" {
			why = message
		}
		findings = append(findings, EvaluatedFinding{
			Line:    max(match.Line, 0),
			Column:  max(match.Column, 0),
			Message: message,
			Why:     why,
		})
	}
	return findings, nil
}

func max(value int, minimum int) int {
	if value < minimum {
		return minimum
	}
	return value
}
