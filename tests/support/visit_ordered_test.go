package support_test

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	checksupport "github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

// withGOMAXPROCS raises GOMAXPROCS for the test so worker pools sized from it
// really run wide (they are also capped by runtime.NumCPU).
func withGOMAXPROCS(t *testing.T, n int) {
	t.Helper()
	previous := runtime.GOMAXPROCS(n)
	t.Cleanup(func() { runtime.GOMAXPROCS(previous) })
}

// TestVisitTargetFilesOrderedMatchesSequentialVisit pins the ordered-visit
// contract: visit runs concurrently, but the commits it returns run on the
// caller's goroutine in listing order, so state built by the commits is
// identical to a sequential VisitTargetFiles pass, run after run.
func TestVisitTargetFilesOrderedMatchesSequentialVisit(t *testing.T) {
	withGOMAXPROCS(t, 32)
	dir := t.TempDir()
	for i := 0; i < 200; i++ {
		writeScanFile(t, dir, fmt.Sprintf("pkg%d/file%03d.txt", i%7, i), strings.Repeat("x", i))
	}
	writeScanFile(t, dir, "skip.bin", "ignored by the include filter")
	sc := runnersupport.Context{}
	target := core.TargetConfig{Name: "repo", Path: dir}
	include := func(rel string) bool { return strings.HasSuffix(rel, ".txt") }

	var want []string
	runnersupport.VisitTargetFiles(sc, target, include, func(rel string, data []byte) {
		want = append(want, fmt.Sprintf("%s:%d", rel, len(data)))
	})
	if len(want) != 200 {
		t.Fatalf("sequential visit saw %d files, want 200", len(want))
	}
	for run := 0; run < 10; run++ {
		var got []string
		var inFlight, maxInFlight atomic.Int32
		runnersupport.VisitTargetFilesOrdered(sc, target, include, func(rel string, data []byte) func() {
			current := inFlight.Add(1)
			for {
				seen := maxInFlight.Load()
				if current <= seen || maxInFlight.CompareAndSwap(seen, current) {
					break
				}
			}
			defer inFlight.Add(-1)
			entry := fmt.Sprintf("%s:%d", rel, len(data))
			return func() { got = append(got, entry) }
		})
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("run %d: ordered visit diverged from sequential visit\ngot:  %v\nwant: %v", run, got, want)
		}
		if maxInFlight.Load() < 1 {
			t.Fatalf("run %d: visit never ran", run)
		}
	}
}

// TestVisitTargetFilesOrderedPropagatesPanic keeps the SafeRun contract for
// the ordered visitor: a panic on a worker resurfaces on the caller.
func TestVisitTargetFilesOrderedPropagatesPanic(t *testing.T) {
	withGOMAXPROCS(t, 16)
	dir := t.TempDir()
	for i := 0; i < 32; i++ {
		writeScanFile(t, dir, fmt.Sprintf("file%02d.txt", i), "content")
	}
	sc := runnersupport.Context{}
	target := core.TargetConfig{Name: "repo", Path: dir}
	recovered := func() (r any) {
		defer func() { r = recover() }()
		runnersupport.VisitTargetFilesOrdered(sc, target, func(string) bool { return true }, func(rel string, _ []byte) func() {
			if rel == "file17.txt" {
				panic("visit boom")
			}
			return nil
		})
		return nil
	}()
	if recovered != "visit boom" {
		t.Fatalf("recovered %v, want the visit panic", recovered)
	}
}

// TestParallelIndexedIsDeterministicAndBounded checks the shared worker pool
// used by the per-file passes: results come back in index order whatever the
// worker count, and no more than the requested workers run at once.
func TestParallelIndexedIsDeterministicAndBounded(t *testing.T) {
	withGOMAXPROCS(t, 32)
	const n = 500
	want := make([]int, n)
	for i := range want {
		want[i] = i * i
	}
	for _, workers := range []int{1, 2, 7, 16, 64} {
		var inFlight, maxInFlight atomic.Int32
		got := checksupport.ParallelIndexed(n, workers, func(i int) int {
			current := inFlight.Add(1)
			for {
				seen := maxInFlight.Load()
				if current <= seen || maxInFlight.CompareAndSwap(seen, current) {
					break
				}
			}
			runtime.Gosched()
			inFlight.Add(-1)
			return i * i
		})
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("workers=%d: results out of order", workers)
		}
		if int(maxInFlight.Load()) > workers {
			t.Fatalf("workers=%d: %d calls ran at once", workers, maxInFlight.Load())
		}
	}
}

// TestParallelWorkersBounds pins the derived limits: at least one worker,
// never more than the items, never more than the usable CPUs, never more than
// the ceiling.
func TestParallelWorkersBounds(t *testing.T) {
	withGOMAXPROCS(t, 64)
	cpus := runtime.NumCPU()
	if got := checksupport.ParallelWorkers(0); got != 1 {
		t.Fatalf("ParallelWorkers(0) = %d, want 1", got)
	}
	if got := checksupport.ParallelWorkers(1); got != 1 {
		t.Fatalf("ParallelWorkers(1) = %d, want 1", got)
	}
	got := checksupport.ParallelWorkers(10_000)
	if got > cpus || got > 16 || got < 1 {
		t.Fatalf("ParallelWorkers(10000) = %d with %d CPUs, want 1..min(cpus,16)", got, cpus)
	}
	runtime.GOMAXPROCS(1)
	if got := checksupport.ParallelWorkers(10_000); got != 1 {
		t.Fatalf("ParallelWorkers under GOMAXPROCS=1 = %d, want 1", got)
	}
	if got := runnersupport.ScanParallelism(); got != 1 {
		t.Fatalf("ScanParallelism under GOMAXPROCS=1 = %d, want 1", got)
	}
}

// TestRunBothPropagatesBackgroundPanic: the overlapped background pass must
// not crash the process; its panic resurfaces on the caller after the
// foreground work completes.
func TestRunBothPropagatesBackgroundPanic(t *testing.T) {
	foregroundRan := false
	recovered := func() (r any) {
		defer func() { r = recover() }()
		checksupport.RunBoth(func() { panic("background boom") }, func() { foregroundRan = true })
		return nil
	}()
	if recovered != "background boom" || !foregroundRan {
		t.Fatalf("recovered %v (foreground ran %t), want the background panic after foreground", recovered, foregroundRan)
	}
}
