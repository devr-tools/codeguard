package quality

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	goErrorfWrapPattern = regexp.MustCompile(`fmt\.Errorf\([^\n]*%w`)
	goErrorfPattern     = regexp.MustCompile(`fmt\.Errorf\(`)
	goErrorsNewPattern  = regexp.MustCompile(`errors\.New\(`)
	goPkgErrorsPattern  = regexp.MustCompile(`(?:^|[^A-Za-z0-9_])(errors\.(?:Wrap|Wrapf|WithStack|WithMessage)\(|github\.com/pkg/errors)(?:$|[^A-Za-z0-9_])`)

	scriptThrowNewPattern = regexp.MustCompile(`throw\s+new\s+([A-Za-z_$][\w$]*)\s*\(`)
)

// PERF: goPkgErrorsPattern opens with the (?:^|[^A-Za-z0-9_]) boundary group,
// which leaves RE2 no literal prefix, so FindAllString attempts a match at
// every byte of every Go file. goRepoStyleProfile runs it over the whole Go
// corpus on the serial critical path, and it was ~80% of that function's time.
// Gate it on the literals every match must contain (the technique of
// quality_text_scan.go and the security line gates).
//
// Soundness: every match contains the capture group, which is one of two
// branches. The first is errors.(Wrap|Wrapf|WithStack|WithMessage)(, so it
// contains "errors.Wrap" (Wrap, Wrapf) or "errors.With" (WithStack,
// WithMessage). The second is the literal "github.com/pkg/errors". The pattern
// has no (?i), so byte-exact strings.Contains is the right comparison, and a
// source holding none of the three literals has zero matches.
//
// The other Go error-style patterns need no gate: each begins with a literal
// (fmt.Errorf( or errors.New(), which RE2 already locates with a substring
// search before it runs the automaton.
var goPkgErrorsLiterals = []string{"errors.Wrap", "errors.With", "github.com/pkg/errors"}

func goPkgErrorsCount(source string) int {
	if !containsAny(source, goPkgErrorsLiterals) {
		return 0
	}
	return len(goPkgErrorsPattern.FindAllString(source, -1))
}

const (
	goErrorStyleWrap      = "fmt.Errorf with %w wrapping"
	goErrorStyleNew       = "errors.New / unwrapped fmt.Errorf"
	goErrorStylePkgErrors = "github.com/pkg/errors wrapping"

	scriptErrorStyleRaw    = "raw throw new Error(...)"
	scriptErrorStyleCustom = "custom error classes"
)

// --- Go error-style drift ---

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
	if pkg := goPkgErrorsCount(source); pkg > 0 {
		counts[goErrorStylePkgErrors] = pkg
	}
	return counts
}

// goErrorStyleDriftFinding is direction-aware: a file whose dominant style is
// %w wrapping is never reported, because wrapping preserves the error chain
// and adopting it in an unwrapped-error repository is an improvement, not
// drift. Unwrapped errors in a %w-dominant repository are still reported.
func goErrorStyleDriftFinding(env support.Context, file string, source string, dominant string) []core.Finding {
	counts := goErrorStyleCounts(source)
	if dominantFrameworkFromCounts(counts) == goErrorStyleWrap {
		return nil
	}
	return errorStyleDriftFinding(env, file, dominant, counts, "error handling")
}

// --- TypeScript/JavaScript error-class drift ---

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

func dominantScriptErrorStyle(env support.Context, target core.TargetConfig, files []string) string {
	return dominantStyle(env, target, files, scriptErrorStyleCounts)
}

func scriptErrorStyleDriftFinding(env support.Context, file string, source string, dominant string) []core.Finding {
	if isLikelyUIFile(file) || isSeedOrScriptSourcePath(file) || strings.Contains(strings.ToLower(file), "/integrations/") {
		return nil
	}
	if scriptUsesSingleCustomErrorClass(source) {
		return nil
	}
	return errorStyleDriftFinding(env, file, dominant, scriptErrorStyleCounts(source), "thrown error")
}

func scriptUsesSingleCustomErrorClass(source string) bool {
	classes := map[string]struct{}{}
	total := 0
	for _, match := range scriptThrowNewPattern.FindAllStringSubmatch(source, -1) {
		if match[1] == "Error" {
			return false
		}
		classes[match[1]] = struct{}{}
		total++
	}
	return total > 0 && len(classes) == 1
}

// --- shared style machinery ---

func dominantStyle(env support.Context, target core.TargetConfig, files []string, counter func(string) map[string]int) string {
	totals := map[string]int{}
	for _, rel := range files {
		data, err := readAITargetFile(env, target, rel)
		if err != nil {
			continue
		}
		for style, count := range counter(string(data)) {
			totals[style] += count
		}
	}
	return dominantStyleFromTotals(totals)
}

// dominantStyleFromTotals picks the highest-count style, requiring a minimum
// amount of repository-wide signal before declaring a dominant style.
func dominantStyleFromTotals(totals map[string]int) string {
	dominant := dominantFrameworkFromCounts(totals)
	if totals[dominant] < 3 {
		return ""
	}
	return dominant
}

func errorStyleDriftFinding(env support.Context, file string, dominant string, counts map[string]int, label string) []core.Finding {
	if dominant == "" {
		return nil
	}
	fileDominant := dominantFrameworkFromCounts(counts)
	if fileDominant == "" || fileDominant == dominant {
		return nil
	}
	if counts[fileDominant] < 2 || counts[dominant] > 0 {
		return nil
	}
	return []core.Finding{newQualityWarnFinding(env, support.WarnFindingInput{
		RuleID:  "quality.ai.error-style-drift",
		Path:    file,
		Line:    1,
		Column:  1,
		Message: fmt.Sprintf("%s style %q diverges from the repository's dominant style %q", label, fileDominant, dominant),
	})}
}
