package quality

import (
	"os"
	"path/filepath"
	"regexp"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

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
		data, err := os.ReadFile(filepath.Join(root, rel)) //nolint:gosec // file under the scan-target root
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
