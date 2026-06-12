package semantic

import (
	"os/exec"
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
	argsVariants := [][]string{
		{"-C", dir, "diff", "--unified=3", "--no-color", baseRef, "--"},
		{"-C", dir, "diff", "--unified=3", "--no-color", baseRef + "...HEAD", "--"},
	}
	for _, args := range argsVariants {
		out, err := exec.Command("git", args...).Output()
		if err == nil {
			return string(out)
		}
	}
	return ""
}
