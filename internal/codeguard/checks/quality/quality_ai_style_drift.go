package quality

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// --- shared naming-convention analysis ---

type nameAt struct {
	name string
	line int
}

type nameExtractor func(source string) []nameAt

var (
	snakeCasePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:_[a-z0-9]+)+$`)
	camelCasePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[A-Z][A-Za-z0-9]*)+$`)
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
func dominantNamingConvention(root string, files []string, extract nameExtractor) string {
	totals := map[string]int{}
	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			continue
		}
		for convention, count := range namingCounts(string(data), extract) {
			totals[convention] += count
		}
	}
	dominant := dominantFrameworkFromCounts(totals)
	if totals[dominant] < 3 {
		return ""
	}
	return dominant
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
	return []core.Finding{env.NewFinding(support.FindingInput{
		RuleID:  "quality.ai.naming-drift",
		Level:   "warn",
		Path:    file,
		Line:    firstDivergent.line,
		Column:  1,
		Message: fmt.Sprintf("identifier %q diverges from the repository's dominant %s naming convention", firstDivergent.name, dominant),
	})}
}

// --- per-language identifier extractors ---

var (
	goFuncNamePattern        = regexp.MustCompile(`(?m)^func\s+(?:\([^)]*\)\s*)?([A-Za-z_][\w]*)\s*\(`)
	pythonDefNamePattern     = regexp.MustCompile(`(?m)^[ \t]*def\s+([A-Za-z_][\w]*)\s*\(`)
	scriptFuncNamePattern    = regexp.MustCompile(`(?m)\bfunction\s+([A-Za-z_$][\w$]*)\s*\(`)
	scriptBindingNamePattern = regexp.MustCompile(`(?m)^[ \t]*(?:export\s+)?(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=`)
)

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
	out := make([]nameAt, 0)
	for _, match := range pattern.FindAllStringSubmatchIndex(source, -1) {
		out = append(out, nameAt{
			name: source[match[2]:match[3]],
			line: 1 + strings.Count(source[:match[2]], "\n"),
		})
	}
	return out
}

// --- Go error-style drift ---

var (
	goErrorfWrapPattern = regexp.MustCompile(`fmt\.Errorf\([^\n]*%w`)
	goErrorfPattern     = regexp.MustCompile(`fmt\.Errorf\(`)
	goErrorsNewPattern  = regexp.MustCompile(`errors\.New\(`)
	goPkgErrorsPattern  = regexp.MustCompile(`errors\.(?:Wrap|Wrapf|WithStack|WithMessage)\(|github\.com/pkg/errors`)
)

const (
	goErrorStyleWrap      = "fmt.Errorf with %w wrapping"
	goErrorStyleNew       = "errors.New / unwrapped fmt.Errorf"
	goErrorStylePkgErrors = "github.com/pkg/errors wrapping"
)

func goErrorStyleCounts(source string) map[string]int {
	counts := map[string]int{}
	wraps := len(goErrorfWrapPattern.FindAllString(source, -1))
	if wraps > 0 {
		counts[goErrorStyleWrap] = wraps
	}
	plain := len(goErrorfPattern.FindAllString(source, -1)) - wraps + len(goErrorsNewPattern.FindAllString(source, -1))
	if plain > 0 {
		counts[goErrorStyleNew] = plain
	}
	if pkg := len(goPkgErrorsPattern.FindAllString(source, -1)); pkg > 0 {
		counts[goErrorStylePkgErrors] = pkg
	}
	return counts
}

func dominantGoErrorStyle(root string, files []string) string {
	return dominantStyle(root, files, goErrorStyleCounts)
}

func goErrorStyleDriftFinding(env support.Context, file string, source string, dominant string) []core.Finding {
	return errorStyleDriftFinding(env, file, source, dominant, goErrorStyleCounts, "error handling")
}

// --- TypeScript/JavaScript error-class drift ---

var scriptThrowNewPattern = regexp.MustCompile(`throw\s+new\s+([A-Za-z_$][\w$]*)\s*\(`)

const (
	scriptErrorStyleRaw    = "raw throw new Error(...)"
	scriptErrorStyleCustom = "custom error classes"
)

func scriptErrorStyleCounts(source string) map[string]int {
	counts := map[string]int{}
	for _, match := range scriptThrowNewPattern.FindAllStringSubmatch(source, -1) {
		if match[1] == "Error" {
			counts[scriptErrorStyleRaw]++
		} else {
			counts[scriptErrorStyleCustom]++
		}
	}
	return counts
}

func dominantScriptErrorStyle(root string, files []string) string {
	return dominantStyle(root, files, scriptErrorStyleCounts)
}

func scriptErrorStyleDriftFinding(env support.Context, file string, source string, dominant string) []core.Finding {
	return errorStyleDriftFinding(env, file, source, dominant, scriptErrorStyleCounts, "thrown error")
}

// --- shared style machinery ---

func dominantStyle(root string, files []string, counter func(string) map[string]int) string {
	totals := map[string]int{}
	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			continue
		}
		for style, count := range counter(string(data)) {
			totals[style] += count
		}
	}
	dominant := dominantFrameworkFromCounts(totals)
	if totals[dominant] < 3 {
		return ""
	}
	return dominant
}

func errorStyleDriftFinding(env support.Context, file string, source string, dominant string, counter func(string) map[string]int, label string) []core.Finding {
	if dominant == "" {
		return nil
	}
	counts := counter(source)
	fileDominant := dominantFrameworkFromCounts(counts)
	if fileDominant == "" || fileDominant == dominant {
		return nil
	}
	if counts[fileDominant] < 2 || counts[dominant] > 0 {
		return nil
	}
	return []core.Finding{env.NewFinding(support.FindingInput{
		RuleID:  "quality.ai.error-style-drift",
		Level:   "warn",
		Path:    file,
		Line:    1,
		Column:  1,
		Message: fmt.Sprintf("%s style %q diverges from the repository's dominant style %q", label, fileDominant, dominant),
	})}
}

// --- Python bare-except drift ---

type pythonErrorStyleSummary struct {
	typedExcepts int
	bareExcepts  int
}

var (
	pythonBareExceptPattern  = regexp.MustCompile(`(?m)^[ \t]*except[ \t]*:`)
	pythonTypedExceptPattern = regexp.MustCompile(`(?m)^[ \t]*except[ \t]+[^\n:]+:`)
)

func pythonErrorStyleCounts(source string) pythonErrorStyleSummary {
	return pythonErrorStyleSummary{
		typedExcepts: len(pythonTypedExceptPattern.FindAllString(source, -1)),
		bareExcepts:  len(pythonBareExceptPattern.FindAllString(source, -1)),
	}
}

func pythonRepoErrorStyle(root string, files []string) pythonErrorStyleSummary {
	total := pythonErrorStyleSummary{}
	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			continue
		}
		counts := pythonErrorStyleCounts(string(data))
		total.typedExcepts += counts.typedExcepts
		total.bareExcepts += counts.bareExcepts
	}
	return total
}

// pythonErrorStyleDriftFindings flags bare except clauses in files when the
// rest of the repository handles exceptions with typed except clauses only.
func pythonErrorStyleDriftFindings(env support.Context, file string, source string, repo pythonErrorStyleSummary) []core.Finding {
	counts := pythonErrorStyleCounts(source)
	if counts.bareExcepts == 0 {
		return nil
	}
	if repo.bareExcepts-counts.bareExcepts > 0 || repo.typedExcepts-counts.typedExcepts < 3 {
		return nil
	}
	findings := make([]core.Finding, 0, counts.bareExcepts)
	for _, line := range regexLineMatches(pythonBareExceptPattern, source) {
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "quality.ai.error-style-drift",
			Level:   "warn",
			Path:    file,
			Line:    line,
			Column:  1,
			Message: "bare except clause diverges from the repository's typed exception handling style",
		}))
	}
	return findings
}
