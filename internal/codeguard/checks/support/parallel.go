package support

import (
	"runtime"
	"sync"
	"sync/atomic"
)

// MaxFileScanWorkers bounds the goroutines one per-file loop may use. Every
// section runs concurrently and each may run several per-file loops at once,
// so the per-loop bound stays small; it is shared by the runner's cached
// per-file scan and by checks that fan out their own per-file work.
const MaxFileScanWorkers = 4

// FileScanWorkers returns the worker count for a per-file loop over items
// entries: at most MaxFileScanWorkers, the CPU count and items, and at least 1.
func FileScanWorkers(items int) int {
	workers := runtime.NumCPU()
	if workers > MaxFileScanWorkers {
		workers = MaxFileScanWorkers
	}
	if workers > items {
		workers = items
	}
	if workers < 1 {
		workers = 1
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
