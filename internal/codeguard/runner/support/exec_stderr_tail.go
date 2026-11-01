package support

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
)

// Bounds for the stderr tail attached to tool-failure diagnostics: enough to
// show a toolchain or module error, never a wall of output.
const (
	StderrTailMaxLines = 20
	StderrTailMaxBytes = 2048
)

// CommandOutput is a bounded subprocess run's result: Combined interleaves
// stdout and stderr exactly as RunLimitedCommand returns them, and StderrTail
// keeps the end of stderr alone so a failure message can show why the tool
// failed.
type CommandOutput struct {
	Combined   string
	StderrTail string
}

// RunLimitedCommandCapture runs name like RunLimitedCommand and also returns
// the last StderrTailMaxBytes of stderr, trimmed to StderrTailMaxLines lines.
func RunLimitedCommandCapture(ctx context.Context, dir string, maxOutputBytes int, name string, args ...string) (CommandOutput, error) {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // caller validates untrusted args before invoking this bounded subprocess helper
	cmd.Dir = dir
	var buf bytes.Buffer
	limited := NewLimitedBufferWriter(&buf, maxOutputBytes)
	combined := &lockedWriter{w: limited}
	tail := &tailBuffer{limit: StderrTailMaxBytes}
	cmd.Stdout = combined
	cmd.Stderr = io.MultiWriter(combined, tail)
	err := cmd.Run()
	out := CommandOutput{StderrTail: TailText(tail.String(), StderrTailMaxLines, StderrTailMaxBytes)}
	if limited.Truncated() {
		return out, fmt.Errorf("%s output exceeded %d bytes", name, maxOutputBytes)
	}
	out.Combined = buf.String()
	return out, err
}

// TailText returns the last maxLines non-blank lines of text, cut to at most
// maxBytes from the end on a line boundary when one is available. It is local
// tool output, so it is shown verbatim, not redacted.
func TailText(text string, maxLines int, maxBytes int) string {
	text = strings.TrimRight(text, " \t\r\n")
	if text == "" {
		return ""
	}
	if maxBytes > 0 && len(text) > maxBytes {
		text = text[len(text)-maxBytes:]
		if newline := strings.IndexByte(text, '\n'); newline >= 0 && newline < len(text)-1 {
			text = text[newline+1:]
		}
	}
	lines := make([]string, 0, maxLines)
	for _, line := range strings.Split(text, "\n") {
		if line = strings.TrimRight(line, " \t\r"); line != "" {
			lines = append(lines, line)
		}
	}
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return strings.Join(lines, "\n")
}

// lockedWriter serialises writes: exec copies stdout and stderr on separate
// goroutines once they are different writers.
type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

// tailBuffer keeps only the last limit bytes written to it.
type tailBuffer struct {
	limit int
	data  []byte
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.data = append(t.data, p...)
	if over := len(t.data) - t.limit; over > 0 {
		t.data = append(t.data[:0], t.data[over:]...)
	}
	return len(p), nil
}

func (t *tailBuffer) String() string {
	return string(t.data)
}
