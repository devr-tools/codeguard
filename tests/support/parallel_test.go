package support_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/config"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

func TestParallelForVisitsEveryIndexOnce(t *testing.T) {
	for _, n := range []int{0, 1, 3, support.MaxFileScanWorkers, 1000} {
		visits := make([]int32, n)
		support.ParallelFor(n, func(i int) {
			atomic.AddInt32(&visits[i], 1)
		})
		for i, count := range visits {
			if count != 1 {
				t.Fatalf("n=%d: index %d visited %d times, want 1", n, i, count)
			}
		}
	}
}

func TestParallelForBoundsConcurrency(t *testing.T) {
	var active, peak atomic.Int32
	support.ParallelFor(200, func(int) {
		now := active.Add(1)
		for {
			seen := peak.Load()
			if now <= seen || peak.CompareAndSwap(seen, now) {
				break
			}
		}
		for i := 0; i < 1000; i++ {
			_ = i * i
		}
		active.Add(-1)
	})
	if got := peak.Load(); got > support.MaxFileScanWorkers {
		t.Fatalf("peak concurrency = %d, want at most %d", got, support.MaxFileScanWorkers)
	}
}

func TestFileScanWorkersClampsToItems(t *testing.T) {
	if got := support.FileScanWorkers(0); got != 1 {
		t.Fatalf("FileScanWorkers(0) = %d, want 1", got)
	}
	if got := support.FileScanWorkers(1); got != 1 {
		t.Fatalf("FileScanWorkers(1) = %d, want 1", got)
	}
	if got := support.FileScanWorkers(1 << 20); got < 1 || got > support.MaxFileScanWorkers {
		t.Fatalf("FileScanWorkers(large) = %d, want 1..%d", got, support.MaxFileScanWorkers)
	}
}

// A panic in a worker must surface on the caller's goroutine, where the
// section's recover can turn it into a diagnostic, instead of crashing the
// process from an unrecovered goroutine.
func TestParallelForReraisesWorkerPanicOnCaller(t *testing.T) {
	defer func() {
		if r := recover(); r != "boom" {
			t.Fatalf("recovered %v, want boom", r)
		}
	}()
	support.ParallelFor(64, func(i int) {
		if i == 13 {
			panic("boom")
		}
	})
	t.Fatal("ParallelFor returned without re-raising the worker panic")
}

func TestRunConcurrentlyRunsEveryTaskAndReraisesPanic(t *testing.T) {
	var ran [3]atomic.Bool
	support.RunConcurrently(
		func() { ran[0].Store(true) },
		func() { ran[1].Store(true) },
		func() { ran[2].Store(true) },
	)
	for i := range ran {
		if !ran[i].Load() {
			t.Fatalf("task %d did not run", i)
		}
	}

	var finished atomic.Bool
	func() {
		defer func() {
			if r := recover(); r != "task failed" {
				t.Fatalf("recovered %v, want task failed", r)
			}
		}()
		support.RunConcurrently(
			func() { panic("task failed") },
			func() { finished.Store(true) },
		)
	}()
	if !finished.Load() {
		t.Fatal("RunConcurrently re-raised before every task finished")
	}
}

// ListTargetFiles hands each caller its own copy of the shared corpus listing.
// Callers sort and normalize the result in place; on the shared slice that
// raced with every concurrent reader and reordered the files every other pass
// iterated.
func TestListTargetFilesReturnsCallerOwnedCopy(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"b.go", "a/z.go", "a-b.go"} {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("package p\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cfg := config.ExampleConfig()
	cfg.Targets = []core.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	sc, err := runnersupport.NewContext(context.Background(), cfg, core.ScanOptions{Mode: core.ScanModeFull})
	if err != nil {
		t.Fatalf("new context: %v", err)
	}
	defer sc.Close()
	target := cfg.Targets[0]
	first, err := runnersupport.ListTargetFiles(sc, target)
	if err != nil || len(first) == 0 {
		t.Fatalf("list: %v (%d files)", err, len(first))
	}
	want := append([]string(nil), first...)
	for i := range first {
		first[i] = "mutated"
	}
	second, err := runnersupport.ListTargetFiles(sc, target)
	if err != nil {
		t.Fatalf("second list: %v", err)
	}
	if !reflect.DeepEqual(second, want) {
		t.Fatalf("listing after caller mutation = %v, want %v", second, want)
	}
}
