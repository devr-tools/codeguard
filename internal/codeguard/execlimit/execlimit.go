// Package execlimit runs subprocesses with a bounded output buffer.
//
// It is a leaf package shared by the runner and the check sections so that
// neither has to reimplement bounded subprocess capture. It performs no trust
// gating: callers that execute config-supplied commands must call
// trust.GuardConfigCommand before invoking these helpers.
package execlimit

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

// Writer writes into a buffer until the byte budget is exhausted, then
// discards the rest while recording truncation.
type Writer struct {
	buffer    *bytes.Buffer
	remaining int
	truncated bool
}

// NewWriter returns a Writer that keeps at most limit bytes in buffer.
func NewWriter(buffer *bytes.Buffer, limit int) *Writer {
	return &Writer{buffer: buffer, remaining: limit}
}

func (writer *Writer) Write(p []byte) (int, error) {
	if writer.remaining <= 0 {
		writer.truncated = true
		return len(p), nil
	}
	if len(p) > writer.remaining {
		writer.buffer.Write(p[:writer.remaining])
		writer.remaining = 0
		writer.truncated = true
		return len(p), nil
	}
	n, err := writer.buffer.Write(p)
	writer.remaining -= n
	return n, err
}

// Truncated reports whether any output was discarded.
func (writer *Writer) Truncated() bool {
	return writer.truncated
}

// Run executes name with args in dir, capturing combined stdout and stderr up
// to maxOutputBytes. A nil or empty env inherits the parent environment. When
// the output exceeds the budget it returns an empty string and an
// "<name> output exceeded <n> bytes" error.
func Run(ctx context.Context, dir string, env []string, maxOutputBytes int, name string, args ...string) (string, error) {
	return run(ctx, dir, env, maxOutputBytes, true, name, args...)
}

// RunStdout is like Run but captures stdout only; stderr is discarded.
func RunStdout(ctx context.Context, dir string, env []string, maxOutputBytes int, name string, args ...string) (string, error) {
	return run(ctx, dir, env, maxOutputBytes, false, name, args...)
}

func run(ctx context.Context, dir string, env []string, maxOutputBytes int, captureStderr bool, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // caller validates untrusted args (and trust-gates config-supplied commands) before invoking this bounded subprocess helper
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = env
	}
	var buf bytes.Buffer
	limited := NewWriter(&buf, maxOutputBytes)
	cmd.Stdout = limited
	if captureStderr {
		cmd.Stderr = limited
	}
	err := cmd.Run()
	if limited.Truncated() {
		return "", fmt.Errorf("%s output exceeded %d bytes", name, maxOutputBytes)
	}
	return buf.String(), err
}
