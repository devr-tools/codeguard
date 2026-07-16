package support

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"context"
)

// gitCommandTimeout is the upper bound on how long a single git invocation may
// run before it is cancelled, layered on top of the caller's context so a hung
// or pathological git process cannot stall a scan indefinitely even when the
// caller never cancels.
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

// runGitCapture runs git with the given args, honouring caller cancellation
// with gitCommandTimeout as an upper bound, and capturing at most
// maxGitOutputBytes of stdout. stderr is captured separately so it can be
// surfaced in errors without counting against the output cap.
func runGitCapture(ctx context.Context, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, gitCommandTimeout)
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

// RunGitCaptureString exposes the bounded git-capture path to packages that
// need raw diff text but should share the same timeout and output cap.
func RunGitCaptureString(ctx context.Context, args ...string) (string, error) {
	out, err := runGitCapture(ctx, args...)
	return string(out), err
}
