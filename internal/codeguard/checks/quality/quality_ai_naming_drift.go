package quality

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type (
	// nameAt is one declared identifier and the line it appears on.
	nameAt struct {
		name string
		line int
	}

	nameExtractor func(source string) []nameAt
)

var (
	snakeCasePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:_[a-z0-9]+)+$`)
	camelCasePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[A-Z][A-Za-z0-9]*)+$`)

	goFuncNamePattern        = regexp.MustCompile(`(?m)^func\s+(?:\([^)]*\)\s*)?([A-Za-z_][\w]*)\s*\(`)
	pythonDefNamePattern     = regexp.MustCompile(`(?m)^[ \t]*def\s+([A-Za-z_][\w]*)\s*\(`)
	scriptFuncNamePattern    = regexp.MustCompile(`(?m)\bfunction\s+([A-Za-z_$][\w$]*)\s*\(`)
	scriptBindingNamePattern = regexp.MustCompile(`(?m)^[ \t]*(?:export\s+)?(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=`)
)

const (
	namingSnake = "snake_case"
	namingCamel = "camelCase"
)

// classifyNamingConvention returns the convention of an identifier, or ""
// when the name carries no signal (single-word names fit every convention).
func classifyNamingConvention(name string) string {
	trimmed := strings.TrimLeft(name, "_")
	switch {
	case snakeCasePattern.MatchString(trimmed):
		return namingSnake
	case camelCasePattern.MatchString(trimmed):
		return namingCamel
	default:
		return ""
	}
}

func namingCounts(source string, extract nameExtractor) map[string]int {
	counts := map[string]int{}
	for _, decl := range extract(source) {
		if convention := classifyNamingConvention(decl.name); convention != "" {
			counts[convention]++
		}
	}
	return counts
}

// dominantNamingConvention establishes the repository-dominant identifier
// convention for the language, requiring a minimum amount of signal.
func dominantNamingConvention(env support.Context, target core.TargetConfig, files []string, extract nameExtractor) string {
	totals := map[string]int{}
	for _, rel := range files {
		data, err := readAITargetFile(env, target, rel)
		if err != nil {
			continue
		}
		for convention, count := range namingCounts(string(data), extract) {
			totals[convention] += count
		}
	}
	return dominantStyleFromTotals(totals)
}

func namingDriftFinding(env support.Context, file string, source string, dominant string, extract nameExtractor) []core.Finding {
	if dominant == "" {
		return nil
	}
	divergent := 0
	matching := 0
	firstDivergent := nameAt{}
	for _, decl := range extract(source) {
		convention := classifyNamingConvention(decl.name)
		switch convention {
		case "":
			continue
		case dominant:
			matching++
		default:
			if divergent == 0 {
				firstDivergent = decl
			}
			divergent++
		}
	}
	if divergent < 2 || divergent <= matching {
		return nil
	}
	return []core.Finding{warnFinding(env, "quality.ai.naming-drift", file, firstDivergent.line, 1,
		fmt.Sprintf("identifier %q diverges from the repository's dominant %s naming convention", firstDivergent.name, dominant))}
}

func goDeclaredNames(source string) []nameAt {
	return extractNames(source, goFuncNamePattern)
}

func pythonDeclaredNames(source string) []nameAt {
	return extractNames(source, pythonDefNamePattern)
}

func scriptDeclaredNames(source string) []nameAt {
	return append(extractNames(source, scriptFuncNamePattern), extractNames(source, scriptBindingNamePattern)...)
}

func extractNames(source string, pattern *regexp.Regexp) []nameAt {
	matches := pattern.FindAllStringSubmatchIndex(source, -1)
	out := make([]nameAt, 0, len(matches))
	for _, match := range matches {
		out = append(out, nameAt{
			name: source[match[2]:match[3]],
			line: 1 + strings.Count(source[:match[2]], "\n"),
		})
	}
	return out
}
