package nlrule

import (
	"crypto/sha1"
	"encoding/hex"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// VerdictCache stores per-evaluation natural-language rule verdicts so an
// unchanged file evaluated by an unchanged rule and runtime never re-invokes
// the runtime. The scan cache implements this interface.
type VerdictCache interface {
	GetNLRuleVerdict(key string) (core.AINLRuleCacheVerdict, bool)
	PutNLRuleVerdict(key string, verdict core.AINLRuleCacheVerdict)
}

// VerdictCacheKey derives the content-hash cache key for one rule evaluation
// from the rule fingerprint, runtime fingerprint, file path, file content
// hash, and prompt version, matching the semantic-cache keying pattern.
func VerdictCacheKey(runtimeFingerprint string, rule core.CustomRuleConfig, path string, data []byte) string {
	contentSum := sha1.Sum(data)
	payload := strings.Join([]string{
		promptVersion,
		ruleFingerprint(rule),
		runtimeFingerprint,
		filepath.ToSlash(path),
		hex.EncodeToString(contentSum[:]),
	}, "|")
	sum := sha1.Sum([]byte("nlrule-verdict-v1|" + payload))
	return hex.EncodeToString(sum[:])
}

func ruleFingerprint(rule core.CustomRuleConfig) string {
	return strings.Join([]string{
		rule.ID,
		rule.Title,
		strings.TrimSpace(rule.Description),
		rule.Message,
		strings.TrimSpace(rule.NaturalLanguage),
	}, "\x1f")
}

func cachedVerdictFromMatches(matches []Match) core.AINLRuleCacheVerdict {
	verdict := core.AINLRuleCacheVerdict{Matches: make([]core.AINLRuleCacheMatch, 0, len(matches))}
	for _, match := range matches {
		verdict.Matches = append(verdict.Matches, core.AINLRuleCacheMatch{
			Line:      match.Line,
			Column:    match.Column,
			Message:   match.Message,
			Rationale: match.Rationale,
		})
	}
	return verdict
}

func matchesFromCachedVerdict(verdict core.AINLRuleCacheVerdict) []Match {
	matches := make([]Match, 0, len(verdict.Matches))
	for _, cached := range verdict.Matches {
		matches = append(matches, Match{
			Line:      cached.Line,
			Column:    cached.Column,
			Message:   cached.Message,
			Rationale: cached.Rationale,
		})
	}
	return matches
}
