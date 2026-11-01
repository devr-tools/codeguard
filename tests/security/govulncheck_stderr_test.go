package security_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	govulncheckrunner "github.com/devr-tools/codeguard/internal/codeguard/runner/govulncheck"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

const toolchainMismatch = "go: go.mod requires go >= 1.99 (running go 1.23; GOTOOLCHAIN=local)"

// installFailingGovulncheck puts a fake govulncheck first on PATH that writes
// noise to stdout and a toolchain error to stderr, then exits 1.
func installFailingGovulncheck(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake govulncheck is a POSIX shell script")
	}
	binDir := t.TempDir()
	script := "#!/bin/sh\necho 'Scanning your code...'\n" +
		"i=0; while [ $i -lt 40 ]; do echo \"noise line $i\" >&2; i=$((i+1)); done\n" +
		"echo 'govulncheck: loading packages:' >&2\necho '" + toolchainMismatch + "' >&2\nexit 1\n"
	if err := os.WriteFile(filepath.Join(binDir, "govulncheck"), []byte(script), 0o755); err != nil {
		t.Fatalf("write fake govulncheck: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func writeGoModule(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/stderr\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}
	return dir
}

func TestGovulncheckFailureCarriesStderrTail(t *testing.T) {
	installFailingGovulncheck(t)
	_, err := govulncheckrunner.Run(context.Background(), writeGoModule(t), "", runnersupport.Context{})
	var failure *govulncheckrunner.FailureError
	if !errors.As(err, &failure) {
		t.Fatalf("expected *FailureError, got %T: %v", err, err)
	}
	if !strings.Contains(failure.StderrTail, toolchainMismatch) {
		t.Fatalf("stderr tail missing the toolchain error:\n%s", failure.StderrTail)
	}
	if strings.Contains(failure.StderrTail, "Scanning your code") {
		t.Fatalf("stderr tail must not include stdout:\n%s", failure.StderrTail)
	}
	if lines := strings.Split(failure.StderrTail, "\n"); len(lines) > runnersupport.StderrTailMaxLines {
		t.Fatalf("stderr tail has %d lines, want at most %d", len(lines), runnersupport.StderrTailMaxLines)
	}
	if strings.Contains(failure.StderrTail, "noise line 0\n") {
		t.Fatalf("stderr tail should keep only the last lines:\n%s", failure.StderrTail)
	}
}

func TestGovulncheckDiagnosticShowsStderr(t *testing.T) {
	installFailingGovulncheck(t)
	_, diagnostics := govulncheckrunner.RunWorkspace(context.Background(), writeGoModule(t), "", runnersupport.Context{})
	for _, diagnostic := range diagnostics {
		if diagnostic.Level != "fail" {
			continue
		}
		if !strings.Contains(diagnostic.Message, "exit status 1") || !strings.Contains(diagnostic.Message, toolchainMismatch) {
			t.Fatalf("diagnostic message should carry exit status and stderr, got:\n%s", diagnostic.Message)
		}
		if !strings.Contains(diagnostic.Metadata["stderr_tail"], toolchainMismatch) {
			t.Fatalf("diagnostic metadata stderr_tail missing toolchain error: %#v", diagnostic.Metadata)
		}
		return
	}
	t.Fatalf("expected a failing govulncheck diagnostic, got %#v", diagnostics)
}

func TestTailTextBounds(t *testing.T) {
	long := strings.Repeat("x", 5000) + "\nlast line\n\n"
	got := runnersupport.TailText(long, 20, 2048)
	if got != "last line" {
		t.Fatalf("TailText should cut to a line boundary within the byte cap, got %q", got)
	}
	lines := make([]string, 0, 30)
	for i := 0; i < 30; i++ {
		lines = append(lines, "l", "")
	}
	if n := len(strings.Split(runnersupport.TailText(strings.Join(lines, "\n"), 20, 0), "\n")); n != 20 {
		t.Fatalf("TailText kept %d lines, want 20 (blank lines dropped)", n)
	}
	if runnersupport.TailText("  \n\n", 20, 2048) != "" {
		t.Fatal("blank input should yield an empty tail")
	}
}
