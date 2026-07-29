package quality

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

const smellRefusedBequestRuleID = "smell.refused-bequest"

var (
	refusedBequestNoopRegexp = regexp.MustCompile(`(?i)\b(unsupported|not\s+implemented|notimplemented|throw\s+new\s+error|raise\s+notimplemented|panic\s*\()`)
	refusedBequestBodyRegexp = regexp.MustCompile(`(?i)\b(?:unsupported|not\s+implemented|notimplemented|not\s+supported|todo\s*\(|panic\s*\(|throw\s+(?:new\s+)?(?:Error|Unsupported|std::runtime_error)|raise\s+(?:NotImplemented|NotImplementedError|RuntimeError))\b`)
	refusedBequestNoopBody   = regexp.MustCompile(`(?i)^\s*(?://.*|/\*.*\*/|#.*)?\s*(?:pass|return\s+None|return\s+nil|return\s+null|return\s+undefined|return\s*;?|continue\s*;?|break\s*;?)\s*$`)
)

func refusedBequestFindings(env support.Context, file string, classes []structuralClass, language string) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, class := range classes {
		bases := uniqueStrings(class.Bases)
		if len(bases) == 0 || len(class.Methods) < 2 {
			continue
		}
		refused := make([]string, 0)
		for _, method := range class.Methods {
			if method.Name == "" || isConstructorLikeMethod(method.Name, class.Name, language) {
				continue
			}
			if methodRefusesInheritedContract(method.Body) {
				refused = append(refused, method.Name)
			}
		}
		refused = uniqueStrings(refused)
		if len(refused) < 2 {
			continue
		}
		sort.Strings(refused)
		bases = sanitizedEvidenceNames(bases)
		methods := sanitizedEvidenceNames(refused)
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:     smellRefusedBequestRuleID,
			Level:      "warn",
			Path:       file,
			Line:       class.StartLine,
			Column:     1,
			Message:    fmt.Sprintf("derived type %s inherits from %s but refuses %d inherited-style methods; prefer composition or split the contract so changes remain safer", class.Name, strings.Join(bases, ", "), len(refused)),
			Confidence: core.ConfidenceHigh,
			Metadata: map[string]string{
				"bases":           strings.Join(bases, ","),
				"refused_methods": strings.Join(methods, ","),
				"refused_count":   fmt.Sprintf("%d", len(refused)),
				"change_signal":   "inheritance-contract-friction",
			},
		}))
	}
	return findings
}

func methodRefusesInheritedContract(body string) bool {
	trimmedLines := make([]string, 0)
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(strings.TrimSuffix(line, ";"))
		if trimmed == "" || trimmed == "{" || trimmed == "}" {
			continue
		}
		trimmedLines = append(trimmedLines, trimmed)
	}
	if len(trimmedLines) == 0 {
		return false
	}
	bodyText := strings.Join(trimmedLines, " ")
	if refusedBequestBodyRegexp.MatchString(bodyText) {
		return true
	}
	if len(trimmedLines) <= 2 {
		for _, line := range trimmedLines {
			if refusedBequestNoopBody.MatchString(line) {
				return true
			}
		}
	}
	return false
}

func isConstructorLikeMethod(methodName string, className string, language string) bool {
	switch language {
	case "python":
		return strings.HasPrefix(methodName, "__") && strings.HasSuffix(methodName, "__")
	default:
		return methodName == className || methodName == "constructor" || strings.HasPrefix(methodName, "~")
	}
}

func sanitizedEvidenceNames(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		value = strings.Trim(value, "*&")
		value = strings.TrimPrefix(value, "public ")
		value = strings.TrimPrefix(value, "private ")
		value = strings.TrimPrefix(value, "protected ")
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return uniqueStrings(out)
}

func parseBaseList(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	parts := splitTopLevelStructuralArgs(text)
	bases := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.HasPrefix(part, "object") || strings.HasPrefix(part, "ABC") {
			continue
		}
		bases = append(bases, part)
	}
	return bases
}

func clikeBaseList(suffix string) []string {
	suffix = strings.TrimSpace(suffix)
	if suffix == "" {
		return nil
	}
	baseText := ""
	if idx := strings.Index(suffix, "extends"); idx >= 0 {
		baseText = suffix[idx+len("extends"):]
		if impl := strings.Index(baseText, "implements"); impl >= 0 {
			baseText = baseText[:impl]
		}
	} else if idx := strings.Index(suffix, ":"); idx >= 0 {
		baseText = suffix[idx+1:]
	} else {
		return nil
	}
	baseText = strings.ReplaceAll(baseText, "public ", "")
	baseText = strings.ReplaceAll(baseText, "private ", "")
	baseText = strings.ReplaceAll(baseText, "protected ", "")
	baseText = strings.ReplaceAll(baseText, "virtual ", "")
	baseText = strings.TrimSpace(baseText)
	return parseBaseList(baseText)
}
