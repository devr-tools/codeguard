package support_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

func writeScanFile(t testing.TB, dir string, name string, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// TestScanTargetFilesDeterministicOrder verifies that the parallel per-file
// worker pool returns findings in exactly the order the sequential walk would,
// run after run: results land in position-indexed slots and are flattened in
// file order, so scheduling must never leak into the output.
func TestScanTargetFilesDeterministicOrder(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 40; i++ {
		writeScanFile(t, dir, fmt.Sprintf("file%02d.txt", i), fmt.Sprintf("content-%02d", i))
	}

	sc := runnersupport.Context{}
	target := core.TargetConfig{Name: "repo", Path: dir}
	include := func(string) bool { return true }
	evaluator := func(file string, data []byte) []core.Finding {
		// Some files intentionally yield no findings so flattening must skip
		// empty slots without disturbing the order of the rest.
		if strings.HasSuffix(file, "5.txt") {
			return nil
		}
		return []core.Finding{{RuleID: "test.rule", Path: file, Message: string(data)}}
	}

	want := runnersupport.ScanTargetFilesSequential(sc, target, "test", include, evaluator)
	if len(want) == 0 {
		t.Fatal("expected sequential scan to produce findings")
	}
	for run := 0; run < 5; run++ {
		got := runnersupport.ScanTargetFiles(sc, target, "test", include, evaluator)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("run %d: parallel scan order diverged from sequential scan:\ngot:  %+v\nwant: %+v", run, got, want)
		}
	}
}

// TestScanTargetFilesPropagatesEvaluatorPanic locks in the safeRun contract:
// an evaluator panic on a worker goroutine must resurface on the calling
// goroutine (where runner/checks downgrades it to a section warning) instead
// of crashing the process from an unrecovered goroutine.
func TestScanTargetFilesPropagatesEvaluatorPanic(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 8; i++ {
		writeScanFile(t, dir, fmt.Sprintf("file%d.txt", i), "content")
	}

	sc := runnersupport.Context{}
	target := core.TargetConfig{Name: "repo", Path: dir}
	recovered := func() (r any) {
		defer func() { r = recover() }()
		runnersupport.ScanTargetFiles(sc, target, "test", func(string) bool { return true }, func(file string, _ []byte) []core.Finding {
			if strings.HasSuffix(file, "3.txt") {
				panic("evaluator boom")
			}
			return nil
		})
		return nil
	}()
	if recovered != "evaluator boom" {
		t.Fatalf("expected evaluator panic to propagate to the caller, got %v", recovered)
	}
}

// TestLoadDiffScopeHonoursCallerCancellation verifies that the caller's
// context now reaches the git subprocess helpers: a pre-cancelled context must
// abort the diff-scope load with context.Canceled rather than running git for
// up to the fixed two-minute timeout.
func TestLoadDiffScopeHonoursCallerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := runnersupport.LoadDiffScope(ctx, []core.TargetConfig{{Name: "repo", Path: t.TempDir()}}, "main")
	if err == nil {
		t.Fatal("expected cancelled context to fail the diff-scope load")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled in error chain, got %v", err)
	}
}

func benchmarkScanTargetFiles(b *testing.B, scan func(runnersupport.Context, core.TargetConfig, string, func(string) bool, func(string, []byte) []core.Finding) []core.Finding) {
	dir := b.TempDir()
	line := strings.Repeat("some scanned line of file content\n", 32)
	for i := 0; i < 64; i++ {
		writeScanFile(b, dir, fmt.Sprintf("file%02d.txt", i), line)
	}

	sc := runnersupport.Context{}
	target := core.TargetConfig{Name: "repo", Path: dir}
	include := func(string) bool { return true }
	evaluator := func(file string, data []byte) []core.Finding {
		lines := strings.Count(string(data), "\n")
		return []core.Finding{{RuleID: "bench.rule", Path: file, Line: lines}}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if findings := scan(sc, target, "bench", include, evaluator); len(findings) != 64 {
			b.Fatalf("expected 64 findings, got %d", len(findings))
		}
	}
}

func BenchmarkScanTargetFiles(b *testing.B) {
	benchmarkScanTargetFiles(b, runnersupport.ScanTargetFiles)
}

func BenchmarkScanTargetFilesSequential(b *testing.B) {
	benchmarkScanTargetFiles(b, runnersupport.ScanTargetFilesSequential)
}
