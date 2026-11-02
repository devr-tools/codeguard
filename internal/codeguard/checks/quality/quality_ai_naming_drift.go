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

	goFuncNameAtStartPattern = regexp.MustCompile(`^func\s+(?:\([^)]*\)\s*)?([A-Za-z_][\w]*)\s*\(`)
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
	return []core.Finding{newQualityWarnFinding(env, support.WarnFindingInput{
		RuleID:  "quality.ai.naming-drift",
		Path:    file,
		Line:    firstDivergent.line,
		Column:  1,
		Message: fmt.Sprintf("identifier %q diverges from the repository's dominant %s naming convention", firstDivergent.name, dominant),
	})}
}

// PERF: goDeclaredNames used to run (?m)^func\s+... over the whole file with
// FindAllStringSubmatchIndex. A leading (?m)^ gives RE2 no literal prefix, so
// the engine tried a match at every byte, and it ran over the whole Go corpus
// in goRepoStyleProfile and again per file. A whole-file strings.Contains gate
// would be sound but useless here, since almost every Go file holds "func".
//
// The gate is positional instead: (?m)^ only matches at offset 0 or directly
// after a '\n', and the next four bytes of every match are the literal
// "func". So only line starts that begin with "func" are tried, each with
// goFuncNameAtStartPattern (the same expression with a plain ^, which RE2
// evaluates at the one position it is given). Scanning resumes at the end of
// each match, exactly as FindAll does, so a line start inside an earlier match
// is skipped in both. Nothing in the expression looks behind its start, so an
// anchored match at a line start is the match FindAll would report there.
func goDeclaredNames(source string) []nameAt {
	var out []nameAt
	line, counted := 1, 0
	for pos := 0; pos < len(source); {
		if (pos == 0 || source[pos-1] == '\n') && strings.HasPrefix(source[pos:], "func") {
			if match := goFuncNameAtStartPattern.FindStringSubmatchIndex(source[pos:]); match != nil {
				nameStart := pos + match[2]
				line += strings.Count(source[counted:nameStart], "\n")
				counted = nameStart
				out = append(out, nameAt{name: source[nameStart : pos+match[3]], line: line})
				pos += match[1]
				continue
			}
		}
		next := strings.IndexByte(source[pos:], '\n')
		if next < 0 {
			break
		}
		pos += next + 1
	}
	return out
}

// PERF: pythonDefNamePattern ((?m)^[ \t]*def), scriptFuncNamePattern
// (\bfunction) and scriptBindingNamePattern ((?m)^[ \t]*...) all open with an
// assertion or a class, so none has a literal prefix. Each is gated on the
// literals every match must contain; all three are case-sensitive, so the
// checks are byte-exact:
//   - pythonDefNamePattern: the literal "def".
//   - scriptFuncNamePattern: the literal "function".
//   - scriptBindingNamePattern: the (?:const|let|var) alternation is
//     mandatory, so every match holds one of its three branches; the gate
//     lists all three.
var (
	pythonDefLiterals     = []string{"def"}
	scriptFuncLiterals    = []string{"function"}
	scriptBindingLiterals = []string{"const", "let", "var"}
)

func pythonDeclaredNames(source string) []nameAt {
	return extractGatedNames(source, pythonDefNamePattern, pythonDefLiterals)
}

func scriptDeclaredNames(source string) []nameAt {
	return append(extractGatedNames(source, scriptFuncNamePattern, scriptFuncLiterals),
		extractGatedNames(source, scriptBindingNamePattern, scriptBindingLiterals)...)
}

func extractGatedNames(source string, pattern *regexp.Regexp, literals []string) []nameAt {
	if !containsAny(source, literals) {
		return nil
	}
	return extractNames(source, pattern)
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
