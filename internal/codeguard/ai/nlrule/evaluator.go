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
	return EvaluateFileCached(ctx, runtime, nil, rule, path, data)
}

// EvaluateFileCached evaluates one natural-language rule against one file,
// serving the verdict from cache when the rule, runtime, and file contents
// are unchanged so the runtime is not re-invoked.
func EvaluateFileCached(ctx context.Context, runtime Runtime, cache VerdictCache, rule core.CustomRuleConfig, path string, data []byte) ([]EvaluatedFinding, error) {
	if runtime == nil || !runtime.Enabled() || strings.TrimSpace(rule.NaturalLanguage) == "" {
		return nil, nil
	}
	key := ""
	if cache != nil {
		key = VerdictCacheKey(runtime.Fingerprint(), rule, path, data)
		if verdict, ok := cache.GetNLRuleVerdict(key); ok {
			return findingsFromMatches(rule, matchesFromCachedVerdict(verdict)), nil
		}
	}
	response, err := runtime.Evaluate(ctx, Compile(rule, path, data))
	if err != nil {
		return nil, err
	}
	if cache != nil {
		cache.PutNLRuleVerdict(key, cachedVerdictFromMatches(response.Matches))
	}
	return findingsFromMatches(rule, response.Matches), nil
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

func max(value int, minimum int) int {
	if value < minimum {
		return minimum
	}
	return value
}
