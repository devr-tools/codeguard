package quality

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

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
	if pkg := len(goPkgErrorsPattern.FindAllString(source, -1)); pkg > 0 {
		counts[goErrorStylePkgErrors] = pkg
	}
	return counts
}

func dominantGoErrorStyle(root string, files []string) string {
	return dominantStyle(root, files, goErrorStyleCounts)
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

func dominantScriptErrorStyle(root string, files []string) string {
	return dominantStyle(root, files, scriptErrorStyleCounts)
}

func scriptErrorStyleDriftFinding(env support.Context, file string, source string, dominant string) []core.Finding {
	return errorStyleDriftFinding(env, file, dominant, scriptErrorStyleCounts(source), "thrown error")
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
	return []core.Finding{env.NewFinding(support.FindingInput{
		RuleID:  "quality.ai.error-style-drift",
		Level:   "warn",
		Path:    file,
		Line:    1,
		Column:  1,
		Message: fmt.Sprintf("%s style %q diverges from the repository's dominant style %q", label, fileDominant, dominant),
	})}
}
