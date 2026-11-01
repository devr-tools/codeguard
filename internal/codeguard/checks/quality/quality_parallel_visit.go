package quality

import (
	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// visitBatchSize is how many files visitTargetFileBatches hands over at once.
// It bounds the file contents held outside the corpus between flushes while
// still giving the per-file worker pool enough work to stay busy.
const visitBatchSize = 256

type visitedFile struct {
	rel  string
	data []byte
}

// visitTargetFileBatches walks the target's files through env.VisitTargetFiles
// — the same listing, order, read-error diagnostics and cache bypass — and
// hands them to flush in visit order, at most visitBatchSize at a time. It
// lets a whole-target pass run its expensive per-file work on a worker pool
// (see support.ParallelFor) while folding the results back in the original
// sequential order.
func visitTargetFileBatches(env support.Context, target core.TargetConfig, include func(string) bool, flush func(batch []visitedFile)) {
	batch := make([]visitedFile, 0, visitBatchSize)
	env.VisitTargetFiles(target, include, func(rel string, data []byte) {
		batch = append(batch, visitedFile{rel: rel, data: data})
		if len(batch) == visitBatchSize {
			flush(batch)
			batch = batch[:0]
		}
	})
	if len(batch) > 0 {
		flush(batch)
	}
}

// visitTargetFilesInParallel is env.VisitTargetFiles with visit run on the
// bounded worker pool. visit may be called concurrently and in any order, so
// it must only accumulate order-independent results under its own
// synchronization.
func visitTargetFilesInParallel(env support.Context, target core.TargetConfig, include func(string) bool, visit func(rel string, data []byte)) {
	visitTargetFileBatches(env, target, include, func(batch []visitedFile) {
		support.ParallelFor(len(batch), func(i int) {
			visit(batch[i].rel, batch[i].data)
		})
	})
}

// fileFindingsInParallel runs analyze for every file on the bounded worker
// pool and concatenates the results in files order, so the output is
// identical to a sequential loop. analyze must only read shared state.
func fileFindingsInParallel(files []string, analyze func(rel string) []core.Finding) []core.Finding {
	slots := make([][]core.Finding, len(files))
	support.ParallelFor(len(files), func(i int) {
		slots[i] = analyze(files[i])
	})
	total := 0
	for _, slot := range slots {
		total += len(slot)
	}
	findings := make([]core.Finding, 0, total)
	for _, slot := range slots {
		findings = append(findings, slot...)
	}
	return findings
}
