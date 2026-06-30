package semantic

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"strings"
	"time"

	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

// gitCommandTimeout bounds a single git invocation, and maxGitOutputBytes caps
// how much diff output is buffered, so a hung process or a huge diff against a
// far-back base ref cannot hang the scan or exhaust memory.
const (
	gitCommandTimeout = 2 * time.Minute
	maxGitOutputBytes = 64 << 20 // 64 MiB
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
	// TODO(harden): thread caller ctx once loadGitDiff accepts one.
	ctx, cancel := context.WithTimeout(context.Background(), gitCommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // fixed git binary; args are tool-built (constants, validated baseRef, target paths)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", false
	}
	if err := cmd.Start(); err != nil {
		return "", false
	}
	var buf bytes.Buffer
	n, _ := io.Copy(&buf, io.LimitReader(stdout, maxGitOutputBytes+1))
	if n > maxGitOutputBytes {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return "", false
	}
	if err := cmd.Wait(); err != nil {
		return "", false
	}
	return buf.String(), true
}
