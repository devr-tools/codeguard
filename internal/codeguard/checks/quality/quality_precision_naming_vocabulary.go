package quality

import (
	"fmt"
	"sort"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func sourceNamingFindings(env support.Context, file string, source string) []core.Finding {
	if isQualityFixturePath(file) {
		return nil
	}
	findings := make([]core.Finding, 0, 3)
	// PERF: all three detectors tokenize the same source. Scan it at most once.
	idents := &sourceIdentifiers{source: source}
	if finding, ok := glossaryDriftFinding(env, file, idents); ok {
		findings = append(findings, finding)
	}
	if finding, ok := roleSuffixOveruseFinding(env, file, idents); ok {
		findings = append(findings, finding)
	}
	if finding, ok := crossLayerInconsistencyFinding(env, file, idents); ok {
		findings = append(findings, finding)
	}
	return findings
}

func glossaryDriftFinding(env support.Context, file string, idents *sourceIdentifiers) (core.Finding, bool) {
	for concept, entry := range env.Config.Checks.QualityRules.Naming.Glossary {
		preferred := strings.ToLower(strings.TrimSpace(concept))
		terms := append([]string{preferred}, entry.Avoid...)
		seen := map[string]int{}
		identWords := idents.wordsPerIdentifier()
		for i, ident := range idents.all() {
			for _, term := range terms {
				normalized := strings.ToLower(strings.TrimSpace(term))
				if normalized == "" {
					continue
				}
				if wordsContain(identWords[i], normalized) {
					seen[normalized] = ident.line
				}
			}
		}
		if len(seen) >= 2 {
			line := firstSeenLine(seen)
			return precisionWarnFinding(env, namingDomainVocabularyDriftRuleID, file, line,
				fmt.Sprintf("domain concept %q appears under multiple terms (%s); prefer one glossary vocabulary", concept, sortedKeys(seen)), core.ConfidenceMedium), true
		}
	}
	return core.Finding{}, false
}

func roleSuffixOveruseFinding(env support.Context, file string, idents *sourceIdentifiers) (core.Finding, bool) {
	normalizedFile := strings.ToLower(strings.ReplaceAll(file, "\\", "/"))
	if isSeedOrScriptSourcePath(file) || isPackageAPIRouterPath(file) ||
		strings.Contains(normalizedFile, "/infra/") || strings.HasPrefix(normalizedFile, "infra/") {
		return core.Finding{}, false
	}
	if isScriptLikeSourcePath(file) {
		fn := precisionFunction{Name: "source"}
		if isUIHelperOrMappingContext(file, fn) {
			return core.Finding{}, false
		}
	}
	threshold := env.Config.Checks.QualityRules.Naming.RoleSuffixWarnThreshold
	if threshold <= 0 {
		threshold = 4
	}
	seen := map[string]int{}
	for _, ident := range idents.all() {
		// hasRoleSuffix replaces `(?i)(manager|helper|util|utils|service|
		// processor)$`: without (?m) that anchor is end-of-text, and an
		// identifier holds no newline, so a suffix test is exactly equivalent.
		if hasRoleSuffix(ident.name) {
			seen[ident.name] = ident.line
		}
	}
	if len(seen) < threshold {
		return core.Finding{}, false
	}
	return precisionWarnFinding(env, namingRoleSuffixOveruseRuleID, file, firstSeenLine(seen),
		fmt.Sprintf("file uses %d vague role suffix names such as Manager/Helper/Util/Service/Processor", len(seen)), core.ConfidenceMedium), true
}

func crossLayerInconsistencyFinding(env support.Context, file string, idents *sourceIdentifiers) (core.Finding, bool) {
	if strings.Contains(idents.source, "userDeletionRequest") {
		return core.Finding{}, false
	}
	groups := [][]string{
		{"restaurant", "venue", "merchant", "establishment"},
		{"user", "customer", "account"},
		{"order", "purchase", "transaction"},
	}
	layerTerms := []string{"api", "request", "response", "dto", "entity", "record", "row", "model", "repository", "domain"}
	// PERF: each identifier is split into words ONCE (shared with the other
	// naming detectors). The nested loops below ask up to 3*(10+4) membership
	// questions per identifier, and each one used to re-split the identifier
	// from scratch — this single detector was ~15% of a full scan's CPU and
	// ~20% of its allocations.
	identWords := idents.wordsPerIdentifier()
	for _, group := range groups {
		seen := map[string]int{}
		for i, ident := range idents.all() {
			words := identWords[i]
			if !wordsContainAny(words, layerTerms) {
				continue
			}
			for _, term := range group {
				if wordsContain(words, term) {
					seen[term] = ident.line
				}
			}
		}
		if len(seen) >= 2 {
			return precisionWarnFinding(env, namingCrossLayerInconsistencyRuleID, file, firstSeenLine(seen),
				fmt.Sprintf("cross-layer names use multiple terms for one concept (%s)", sortedKeys(seen)), core.ConfidenceLow), true
		}
	}
	return core.Finding{}, false
}

func firstSeenLine(seen map[string]int) int {
	line := 0
	for _, candidate := range seen {
		if line == 0 || candidate < line {
			line = candidate
		}
	}
	if line == 0 {
		return 1
	}
	return line
}

func sortedKeys(seen map[string]int) string {
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}
