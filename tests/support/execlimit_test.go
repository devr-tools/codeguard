package support_test

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/execlimit"
)

func TestExeclimitWriterTruncatesPastBudget(t *testing.T) {
	var buf bytes.Buffer
	w := execlimit.NewWriter(&buf, 4)
	n, err := w.Write([]byte("abcdef"))
	if err != nil || n != 6 {
		t.Fatalf("Write() = (%d, %v), want (6, nil)", n, err)
	}
	if !w.Truncated() || buf.String() != "abcd" {
		t.Fatalf("truncated=%v buf=%q, want true %q", w.Truncated(), buf.String(), "abcd")
	}
	if n, err := w.Write([]byte("more")); err != nil || n != 4 || buf.String() != "abcd" {
		t.Fatalf("second Write() = (%d, %v) buf=%q", n, err, buf.String())
	}
}

func TestExeclimitWriterWithinBudget(t *testing.T) {
	var buf bytes.Buffer
	w := execlimit.NewWriter(&buf, 4)
	if n, err := w.Write([]byte("abcd")); err != nil || n != 4 {
		t.Fatalf("Write() = (%d, %v), want (4, nil)", n, err)
	}
	if w.Truncated() {
		t.Fatal("writer reported truncation for output exactly at the budget")
	}
}

func requireShell(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}
}

func TestExeclimitRunCapturesCombinedOutput(t *testing.T) {
	requireShell(t)
	out, err := execlimit.Run(context.Background(), t.TempDir(), nil, 1<<10, "sh", "-c", "printf out; printf err >&2")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(out, "out") || !strings.Contains(out, "err") {
		t.Fatalf("Run() output = %q, want stdout and stderr", out)
	}
}

func TestExeclimitRunStdoutDiscardsStderr(t *testing.T) {
	requireShell(t)
	out, err := execlimit.RunStdout(context.Background(), "", nil, 1<<10, "sh", "-c", "printf out; printf err >&2")
	if err != nil || out != "out" {
		t.Fatalf("RunStdout() = (%q, %v), want (\"out\", nil)", out, err)
	}
}

func TestExeclimitRunReportsOverflow(t *testing.T) {
	requireShell(t)
	out, err := execlimit.Run(context.Background(), "", nil, 3, "sh", "-c", "printf abcdef")
	if out != "" || err == nil || err.Error() != "sh output exceeded 3 bytes" {
		t.Fatalf("Run() = (%q, %v), want empty output and overflow error", out, err)
	}
}

func TestExeclimitRunUsesProvidedEnv(t *testing.T) {
	requireShell(t)
	shPath, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh not available")
	}
	out, err := execlimit.Run(context.Background(), "", []string{"EXECLIMIT_PROBE=yes"}, 1<<10, shPath, "-c", "printf \"$EXECLIMIT_PROBE\"")
	if err != nil || out != "yes" {
		t.Fatalf("Run() = (%q, %v), want (\"yes\", nil)", out, err)
	}
}
