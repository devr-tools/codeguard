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

// FileEvaluation bundles the rule and the file it is evaluated against.
type FileEvaluation struct {
	Rule core.CustomRuleConfig
	Path string
	Data []byte
}

func EvaluateFile(ctx context.Context, runtime Runtime, rule core.CustomRuleConfig, path string, data []byte) ([]EvaluatedFinding, error) {
	return EvaluateFileCached(ctx, runtime, nil, FileEvaluation{Rule: rule, Path: path, Data: data})
}

// EvaluateFileCached evaluates one natural-language rule against one file,
// serving the verdict from cache when the rule, runtime, and file contents
// are unchanged so the runtime is not re-invoked.
func EvaluateFileCached(ctx context.Context, runtime Runtime, cache VerdictCache, eval FileEvaluation) ([]EvaluatedFinding, error) {
	if runtime == nil || !runtime.Enabled() || strings.TrimSpace(eval.Rule.NaturalLanguage) == "" {
		return nil, nil
	}
	key := ""
	if cache != nil {
		key = VerdictCacheKey(runtime.Fingerprint(), eval.Rule, eval.Path, eval.Data)
		if verdict, ok := cache.GetNLRuleVerdict(key); ok {
			return findingsFromMatches(eval.Rule, matchesFromCachedVerdict(verdict)), nil
		}
	}
	response, err := runtime.Evaluate(ctx, Compile(eval.Rule, eval.Path, eval.Data))
	if err != nil {
		return nil, err
	}
	if cache != nil {
		cache.PutNLRuleVerdict(key, cachedVerdictFromMatches(response.Matches))
	}
	return findingsFromMatches(eval.Rule, response.Matches), nil
}

func findingsFromMatches(rule core.CustomRuleConfig, matches []Match) []EvaluatedFinding {
	findings := make([]EvaluatedFinding, 0, len(matches))
	for _, match := range matches {
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
	return findings
}
