package quality

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func parsedDuplicatedKnowledgeFindings(env support.Context, file string, parsed *support.ParsedFile) []core.Finding {
	if isQualityFixturePath(file) {
		return nil
	}
	if strings.HasPrefix(strings.ToLower(file), ".buildkite/") || isSeedOrScriptSourcePath(file) {
		return nil
	}
	seen := map[string]int{}
	for _, statement := range parsed.Module.Statements {
		for _, literal := range domainKnowledgeLiterals(statement.Raw) {
			if first, exists := seen[literal]; exists {
				return []core.Finding{precisionWarnFinding(env, qualityDuplicatedKnowledgeRuleID, file, statement.Line,
					fmt.Sprintf("business literal %s is duplicated near line %d; centralize shared domain knowledge", literal, first), core.ConfidenceLow)}
			}
			seen[literal] = statement.Line
		}
	}
	return nil
}

func sourceDuplicatedKnowledgeFindings(env support.Context, file string, source string) []core.Finding {
	if isQualityFixturePath(file) {
		return nil
	}
	if strings.HasPrefix(strings.ToLower(file), ".buildkite/") || isSeedOrScriptSourcePath(file) {
		return nil
	}
	seen := map[string]int{}
	for idx, line := range strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		for _, literal := range domainKnowledgeLiterals(line) {
			if first, exists := seen[literal]; exists {
				return []core.Finding{precisionWarnFinding(env, qualityDuplicatedKnowledgeRuleID, file, idx+1,
					fmt.Sprintf("business literal %s is duplicated near line %d; centralize shared domain knowledge", literal, first), core.ConfidenceLow)}
			}
			seen[literal] = idx + 1
		}
	}
	return nil
}

func domainKnowledgeLiterals(line string) []string {
	if duplicatedKnowledgeLineIsDisplayOnly(line) || duplicatedKnowledgeLineIsStructural(line) {
		return nil
	}
	matches := regexp.MustCompile(`"([^"]{2,80})"|'([^']{2,80})'|\b\d+(?:\.\d+)?\b`).FindAllString(line, -1)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		if domainKnowledgeLiteralInLine(match, line) {
			out = append(out, match)
		}
	}
	return out
}

func duplicatedKnowledgeLineIsDisplayOnly(line string) bool {
	lowered := strings.ToLower(line)
	if strings.Contains(lowered, "classname") || strings.Contains(lowered, "clasname") || strings.Contains(lowered, "class:") {
		return true
	}
	if strings.Contains(lowered, "class") && strings.Contains(line, "-") {
		return true
	}
	for _, marker := range []string{"css", "style", "styles", "variant", "variants", "tailwind", "stylesheet"} {
		if strings.Contains(lowered, marker) {
			return true
		}
	}
	if strings.Contains(line, "<") && strings.Contains(line, ">") {
		return true
	}
	if strings.Contains(lowered, "label:") || strings.Contains(lowered, "placeholder:") || strings.Contains(lowered, "title:") ||
		strings.Contains(lowered, "aria-label") {
		return true
	}
	return false
}

func duplicatedKnowledgeLineIsStructural(line string) bool {
	trimmed := strings.TrimSpace(line)
	lowered := strings.ToLower(trimmed)
	if strings.HasPrefix(trimmed, "import ") || strings.HasPrefix(trimmed, "export {") ||
		strings.HasPrefix(trimmed, "export *") || strings.Contains(lowered, " from ") ||
		strings.Contains(lowered, "require(") {
		return true
	}
	if strings.Contains(lowered, "content-type") || strings.Contains(lowered, "authorization") ||
		strings.Contains(lowered, "accept:") || strings.Contains(lowered, "headers") {
		return true
	}
	return false
}

func domainKnowledgeLiteral(value string) bool {
	return domainKnowledgeLiteralInLine(value, "")
}

func domainKnowledgeLiteralInLine(value string, line string) bool {
	trimmed := strings.Trim(value, `"'`)
	if trimmed == "" || len(trimmed) > 80 {
		return false
	}
	if len(trimmed) > 48 {
		return false
	}
	if strings.Contains(trimmed, "-") {
		return false
	}
	if strings.Contains(trimmed, "${") || strings.Contains(trimmed, "________________") || strings.Contains(trimmed, "__test-stubs__") {
		return false
	}
	if strings.Contains(trimmed, "'") || strings.Contains(trimmed, ". ") || strings.Contains(trimmed, ", ") {
		return false
	}
	if len(trimmed) < 4 && !strings.ContainsAny(trimmed, "0123456789") {
		return false
	}
	if numeric, ok := duplicatedKnowledgeNumber(trimmed); ok {
		_ = numeric
		return false
	}
	if duplicatedKnowledgeStyleLiteral(trimmed) {
		return false
	}
	if duplicatedKnowledgeSentinelLiteral(trimmed) {
		return false
	}
	if duplicatedKnowledgeTableOrEnumLiteral(trimmed, line) {
		return false
	}
	if duplicatedKnowledgeEnumStatusLiteral(trimmed, line) {
		return false
	}
	if likelyDisplayLabel(trimmed) {
		return false
	}
	return domainPrimitiveNamePattern.MatchString(trimmed) || strings.Contains(trimmed, "_")
}

func duplicatedKnowledgeStyleLiteral(value string) bool {
	lowered := strings.ToLower(strings.TrimSpace(value))
	return strings.Contains(lowered, "var(--") ||
		strings.Contains(lowered, "bg-") ||
		strings.Contains(lowered, "text-") ||
		strings.Contains(lowered, "border-") ||
		lowered == "_blank" ||
		lowered == "content-type" ||
		lowered == "authorization" ||
		lowered == "application/json"
}

func duplicatedKnowledgeNumericLiteral(number int, line string) bool {
	if number < 0 {
		number = -number
	}
	if number < 100 {
		return false
	}
	lowered := strings.ToLower(line)
	return line == "" ||
		domainPrimitiveNamePattern.MatchString(lowered) ||
		durationNamePattern.MatchString(lowered) ||
		sizeNamePattern.MatchString(lowered) ||
		moneyNamePattern.MatchString(lowered)
}

func duplicatedKnowledgeEnumStatusLiteral(value string, line string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	enumLike := strings.Contains(trimmed, "_") || strings.ToUpper(trimmed) == trimmed
	if !enumLike {
		return false
	}
	if looksLikeAllCapsEnumLiteral(trimmed) {
		return true
	}
	if line == "" {
		return false
	}
	loweredLine := strings.ToLower(line)
	for _, marker := range []string{"enum", "status", "type:", "kind:", "value:", "option", "label", "as const", "satisfies"} {
		if strings.Contains(loweredLine, marker) {
			return true
		}
	}
	return false
}

func duplicatedKnowledgeSentinelLiteral(value string) bool {
	trimmed := strings.TrimSpace(value)
	if strings.HasPrefix(trimmed, "__") && len(trimmed) <= 40 {
		return true
	}
	lowered := strings.ToLower(trimmed)
	return containsAny(lowered, []string{
		"agent_stop", "customer_user", "invalid_response", "invalid_shape", "last_week", "max_tokens", "missing_body",
		"no_email", "no_json", "no_token", "not_in_workspace", "response_too_large", "score_risk",
		"regulatory_category", "this_month", "this_week", "this_year", "last_30_days", "last_90_days",
	})
}

func duplicatedKnowledgeTableOrEnumLiteral(value string, line string) bool {
	if !strings.Contains(value, "_") {
		return false
	}
	if len(value) > 48 {
		return false
	}
	parts := strings.Split(value, "_")
	if len(parts) > 4 {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
	}
	loweredLine := strings.ToLower(line)
	return containsAny(loweredLine, []string{"table", "tablename", "table_name", "enum", "status", "type", "kind", "key:", "value:", "option"})
}

func looksLikeAllCapsEnumLiteral(value string) bool {
	hasLetter := false
	for _, r := range value {
		switch {
		case r >= 'A' && r <= 'Z':
			hasLetter = true
		case r >= '0' && r <= '9':
			continue
		case r == '_' || r == '-' || r == ':':
			continue
		default:
			return false
		}
	}
	return hasLetter && strings.ToUpper(value) == value
}

func duplicatedKnowledgeNumber(value string) (int, bool) {
	number, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	if number < 0 {
		number = -number
	}
	return number, true
}

func likelyDisplayLabel(value string) bool {
	if strings.Contains(value, "_") {
		return false
	}
	if strings.ContainsAny(value, "-:.") {
		return false
	}
	words := strings.Fields(value)
	return len(words) > 0 && len(words) <= 6
}
