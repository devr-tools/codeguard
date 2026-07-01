package support

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// gitCommandTimeout bounds how long a single git invocation may run before it
// is cancelled. It guards against a hung or pathological git process when no
// caller context is available to thread through.
const gitCommandTimeout = 2 * time.Minute

// maxGitOutputBytes caps how much stdout codeguard will buffer from a git
// subprocess. A diff against a far-back base ref can be hundreds of MB; reading
// it unbounded risks exhausting memory, so output past this cap is an error.
const maxGitOutputBytes = 64 << 20 // 64 MiB

// errGitOutputTooLarge is returned when a git subprocess produces more output
// than maxGitOutputBytes.
var errGitOutputTooLarge = fmt.Errorf("git output exceeded %d bytes", maxGitOutputBytes)

// validBaseRef reports whether ref is a safe value to pass to git as a
// revision/ref:path argument. It rejects refs beginning with "-" (which git
// would otherwise parse as an option even after "--") and restricts the value
// to a conservative ref/SHA charset. The literal "stdin" sentinel (used when a
// diff is supplied directly rather than read from git) is always allowed.
func validBaseRef(ref string) bool {
	if ref == "stdin" {
		return true
	}
	if ref == "" || strings.HasPrefix(ref, "-") {
		return false
	}
	for _, r := range ref {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9':
			continue
		}
		switch r {
		case '.', '_', '/', '-', '~', '^', '@', '{', '}', ':':
			continue
		default:
			return false
		}
	}
	return true
}

// ValidateBaseRef validates a base ref at the trust boundary, returning a clear
// error when the ref could be misinterpreted by git as an option or contains
// unexpected characters.
func ValidateBaseRef(ref string) error {
	if !validBaseRef(ref) {
		return fmt.Errorf("invalid base ref %q", ref)
	}
	return nil
}

// runGitCapture runs git with the given args, enforcing gitCommandTimeout and
// capturing at most maxGitOutputBytes of stdout. stderr is captured separately
// so it can be surfaced in errors without counting against the output cap.
func runGitCapture(args ...string) ([]byte, error) {
	// TODO(harden): thread caller ctx once the diff helpers accept one.
	ctx, cancel := context.WithTimeout(context.Background(), gitCommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // fixed git binary; args are tool-built (constants, validated baseRef, target paths)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	n, copyErr := io.Copy(&buf, io.LimitReader(stdout, maxGitOutputBytes+1))
	if n > maxGitOutputBytes {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, errGitOutputTooLarge
	}
	if waitErr := cmd.Wait(); waitErr != nil {
		if stderr.Len() > 0 {
			return buf.Bytes(), fmt.Errorf("%w: %s", waitErr, strings.TrimSpace(stderr.String()))
		}
		return buf.Bytes(), waitErr
	}
	if copyErr != nil {
		return buf.Bytes(), copyErr
	}
	return buf.Bytes(), nil
}

type LineRanges struct {
	allChanged bool
	ranges     [][2]int
}

// Export converts the internal representation into the core type shared with
// checks that need to intersect findings with changed lines.
func (r LineRanges) Export() core.ChangedLineRanges {
	return core.ChangedLineRanges{
		AllChanged: r.allChanged,
		Ranges:     append([][2]int(nil), r.ranges...),
	}
}

func LoadDiffScope(targets []core.TargetConfig, baseRef string) (map[string]LineRanges, error) {
	out := map[string]LineRanges{}
	for _, target := range targets {
		scope, err := gitChangedLines(target.Path, baseRef)
		if err != nil {
			return nil, err
		}
		for path, ranges := range scope {
			out[path] = ranges
		}
	}
	return out, nil
}

func gitChangedLines(dir string, baseRef string) (map[string]LineRanges, error) {
	if err := ValidateBaseRef(baseRef); err != nil {
		return nil, err
	}
	argsVariants := [][]string{
		{"-C", dir, "diff", "--unified=0", "--no-color", "--end-of-options", baseRef, "--"},
		{"-C", dir, "diff", "--unified=0", "--no-color", "--end-of-options", baseRef + "...HEAD", "--"},
	}
	var output []byte
	var err error
	for _, args := range argsVariants {
		output, err = runGitCapture(args...)
		if err == nil {
			return parseUnifiedDiff(string(output)), nil
		}
	}
	return nil, fmt.Errorf("diff mode requires git diff against %q: %w", baseRef, err)
}

func parseUnifiedDiff(diff string) map[string]LineRanges {
	out := map[string]LineRanges{}
	currentFile := ""
	deletedFrom := ""
	lines := strings.Split(diff, "\n")
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "--- a/"):
			deletedFrom = strings.TrimPrefix(line, "--- a/")
		case strings.HasPrefix(line, "+++ /dev/null"):
			// Deleted file: keep the old path in scope so findings that
			// reference removed files survive diff filtering.
			currentFile = ""
			if deletedFrom != "" {
				out[deletedFrom] = LineRanges{allChanged: true}
				deletedFrom = ""
			}
		case strings.HasPrefix(line, "+++ b/"):
			deletedFrom = ""
			currentFile = strings.TrimPrefix(line, "+++ b/")
			if currentFile != "" {
				if _, ok := out[currentFile]; !ok {
					out[currentFile] = LineRanges{allChanged: true}
				}
			}
		case strings.HasPrefix(line, "@@") && currentFile != "":
			start, end, ok := parseHunkHeader(line)
			if !ok {
				continue
			}
			scope := out[currentFile]
			scope.allChanged = false
			scope.ranges = append(scope.ranges, [2]int{start, end})
			out[currentFile] = scope
		}
	}
	return out
}

func parseHunkHeader(header string) (int, int, bool) {
	parts := strings.Split(header, " ")
	for _, part := range parts {
		if !strings.HasPrefix(part, "+") {
			continue
		}
		part = strings.TrimPrefix(part, "+")
		part = strings.TrimSuffix(part, "@@")
		pieces := strings.Split(part, ",")
		start, err := strconv.Atoi(strings.TrimSpace(pieces[0]))
		if err != nil {
			return 0, 0, false
		}
		count := 1
		if len(pieces) > 1 {
			count, _ = strconv.Atoi(strings.TrimSpace(pieces[1]))
		}
		if count == 0 {
			return start, start, true
		}
		return start, start + count - 1, true
	}
	return 0, 0, false
}
