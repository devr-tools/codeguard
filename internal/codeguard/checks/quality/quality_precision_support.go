package quality

import (
	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type precisionLineRange struct {
	Start int
	End   int
}

func precisionWarnFinding(env support.Context, ruleID string, file string, line int, message string, confidence string) core.Finding {
	return precisionWarnFindingWithMetadata(env, ruleID, file, line, message, confidence, nil)
}

func precisionWarnFindingWithMetadata(env support.Context, ruleID string, file string, line int, message string, confidence string, metadata map[string]string) core.Finding {
	return env.NewFinding(support.FindingInput{
		RuleID:     ruleID,
		Level:      "warn",
		Path:       file,
		Line:       line,
		Column:     1,
		Message:    message,
		Confidence: confidence,
		Metadata:   metadata,
	})
}

func nestedPrecisionLineRanges(fn *support.ParsedFunction) []precisionLineRange {
	out := make([]precisionLineRange, 0, len(fn.Nested))
	var collect func(items []*support.ParsedFunction)
	collect = func(items []*support.ParsedFunction) {
		for _, nested := range items {
			out = append(out, precisionLineRange{Start: nested.StartLine, End: nested.EndLine})
			collect(nested.Nested)
		}
	}
	collect(fn.Nested)
	return out
}
