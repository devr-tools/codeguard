package support

import (
	"runtime"
	"sync"
	"sync/atomic"
)

// maxParallelWorkers caps the per-pass worker count however wide the host:
// each worker holds one file's source and analysis state, so the worker count
// bounds that part of peak memory.
const maxParallelWorkers = 16

// ParallelWorkers is the worker count for a pass over n independent items: one
// per usable CPU (GOMAXPROCS, never more than runtime.NumCPU), at most
// maxParallelWorkers, and never more than n. Setting the GOMAXPROCS
// environment variable narrows every pass.
func ParallelWorkers(n int) int {
	workers := runtime.GOMAXPROCS(0)
	if cpus := runtime.NumCPU(); cpus < workers {
		workers = cpus
	}
	if workers > maxParallelWorkers {
		workers = maxParallelWorkers
	}
	if workers > n {
		workers = n
	}
	if workers < 1 {
		workers = 1
	}
	return workers
}

// ParallelIndexed runs fn(0..n-1) on up to workers goroutines and returns the
// results in index order, so the output never depends on scheduling. With one
// worker it runs inline. A panic in any call stops the remaining work and is
// re-raised on the caller's goroutine once every worker has returned, where
// the section-level recovery (runner/checks.SafeRun) turns it into a failing
// diagnostic instead of crashing the process.
func ParallelIndexed[T any](n int, workers int, fn func(i int) T) []T {
	slots := make([]T, n)
	if workers <= 1 || n <= 1 {
		for i := range slots {
			slots[i] = fn(i)
		}
		return slots
	}
	var next atomic.Int64
	var panicked atomic.Bool
	var panicValue any
	var panicOnce sync.Once
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicOnce.Do(func() {
						panicValue = r
						panicked.Store(true)
					})
				}
			}()
			for !panicked.Load() {
				i := int(next.Add(1)) - 1
				if i >= n {
					return
				}
				slots[i] = fn(i)
			}
		}()
	}
	wg.Wait()
	if panicked.Load() {
		panic(panicValue)
	}
	return slots
}

// RunBoth runs background on a new goroutine while foreground runs on the
// caller's, and returns once both have finished. A panic in background is
// re-raised on the caller's goroutine after foreground returns, so the
// section-level recovery still applies.
//
// It is for overlapping independent work inside one section, such as a
// CPU-bound file pass with a pass that mostly waits on a subprocess.
func RunBoth(background func(), foreground func()) {
	var wg sync.WaitGroup
	var recovered any
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() { recovered = recover() }()
		background()
	}()
	defer func() {
		wg.Wait()
		if recovered != nil {
			panic(recovered)
		}
	}()
	foreground()
}

// MaxFileScanWorkers bounds the goroutines one per-file loop may use. Every
// section runs concurrently and each may run several per-file loops at once,
// so the per-loop bound stays small. The runner's own per-file scan is a single
// loop per section and uses ParallelWorkers instead.
const MaxFileScanWorkers = 4

// FileScanWorkers returns the worker count for a per-file loop nested inside a
// section's concurrent passes: ParallelWorkers(items), further capped at
// MaxFileScanWorkers because several such loops run at once.
func FileScanWorkers(items int) int {
	workers := ParallelWorkers(items)
	if workers > MaxFileScanWorkers {
		workers = MaxFileScanWorkers
	}
	return workers
}

// ParallelFor calls fn(i) for every i in [0, n) on a bounded worker pool (see
// FileScanWorkers) and returns once every call has finished. Calls run in no
// particular order, so fn must write only to state owned by index i — callers
// that need ordered output write into a slot per index and merge afterwards.
// A panic in fn is re-raised on the calling goroutine, so it is still caught
// by the section's recover rather than crashing the process.
func ParallelFor(n int, fn func(i int)) {
	workers := FileScanWorkers(n)
	if workers <= 1 {
		for i := 0; i < n; i++ {
			fn(i)
		}
		return
	}
	var next atomic.Int64
	var group panicGroup
	for range workers {
		group.Go(func() {
			for {
				i := int(next.Add(1)) - 1
				if i >= n {
					return
				}
				fn(i)
			}
		})
	}
	group.Wait()
}

// RunConcurrently runs every task on its own goroutine and waits for all of
// them. It is for a small, fixed set of independent passes; per-item fan-out
// belongs in ParallelFor, which bounds its workers. Each task must write only
// to state it owns. A panic in a task is re-raised on the calling goroutine
// after every task has finished.
func RunConcurrently(tasks ...func()) {
	if len(tasks) == 1 {
		tasks[0]()
		return
	}
	var group panicGroup
	for _, task := range tasks {
		group.Go(task)
	}
	group.Wait()
}

// panicGroup is a WaitGroup that captures the first panic raised by any of
// its goroutines and re-raises it from Wait.
type panicGroup struct {
	wg    sync.WaitGroup
	once  sync.Once
	value any
	set   atomic.Bool
}

func (g *panicGroup) Go(fn func()) {
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				g.once.Do(func() {
					g.value = r
					g.set.Store(true)
				})
			}
		}()
		fn()
	}()
}

func (g *panicGroup) Wait() {
	g.wg.Wait()
	if g.set.Load() {
		panic(g.value)
	}
}
