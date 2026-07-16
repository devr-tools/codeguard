package semantic

import (
	"context"
	"strings"

	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

func changedFilesFromDiff(diffText string) []string {
	return runnersupport.ChangedFilesFromUnifiedDiff(diffText)
}

func loadGitDiff(dir string, baseRef string) string {
	if strings.TrimSpace(baseRef) == "" {
		return ""
	}
	if err := runnersupport.ValidateBaseRef(baseRef); err != nil {
		return ""
	}
	argsVariants := [][]string{
		{"-C", dir, "diff", "--unified=3", "--no-color", "--end-of-options", baseRef, "--"},
		{"-C", dir, "diff", "--unified=3", "--no-color", "--end-of-options", baseRef + "...HEAD", "--"},
	}
	for _, args := range argsVariants {
		if out, ok := runGitDiffCapture(args...); ok {
			return out
		}
	}
	return ""
}

// runGitDiffCapture runs git with a timeout and reads at most maxGitOutputBytes
// of stdout. It reports ok=false when git fails or the output cap is exceeded.
func runGitDiffCapture(args ...string) (string, bool) {
	out, err := runnersupport.RunGitCaptureString(context.Background(), args...)
	if err != nil {
		return "", false
	}
	return out, true
}
