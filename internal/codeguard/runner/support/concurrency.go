package support

import (
	"runtime"

	checkSupport "github.com/devr-tools/codeguard/internal/codeguard/checks/support"
)

// Scan concurrency is derived from the CPUs the process may use rather than
// fixed. The previous fixed caps (two concurrent corpus reads, two Go parses,
// four file workers per section, and several whole-target passes that ran on
// one goroutine) left most of an 8-core runner idle: sections
// queued on the shared read gate, and a section's file pass could never use
// more than half the machine even when it was the only section left running.
//
// Every limit has a floor, so a single-CPU container still overlaps a read
// with analysis, and a ceiling, because each in-flight read or parse holds up
// to maxScanFileBytes of source (plus, for a parse, its transient working set)
// and the number of those buffers is what bounds peak memory, not the CPU
// count. The ceilings keep the worst case on a very wide host the same order
// of magnitude as on a CI runner.
const (
	minCorpusReadWorkers = 2
	maxCorpusReadWorkers = 16
	minGoParseWorkers    = 2
	maxGoParseWorkers    = 16
)

// scanParallelism is the number of goroutines the scan can usefully keep
// running: GOMAXPROCS, which defaults to the CPU count, so setting the
// GOMAXPROCS environment variable is the operator's knob for narrowing a scan
// (for example to a container's CPU quota). It never exceeds runtime.NumCPU.
func scanParallelism() int {
	procs := runtime.GOMAXPROCS(0)
	if cpus := runtime.NumCPU(); cpus < procs {
		procs = cpus
	}
	if procs < 1 {
		procs = 1
	}
	return procs
}

// ScanParallelism exposes scanParallelism to the section scheduler.
func ScanParallelism() int { return scanParallelism() }

func clampWorkers(n int, lo int, hi int) int {
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}

func corpusReadWorkers() int {
	return clampWorkers(scanParallelism(), minCorpusReadWorkers, maxCorpusReadWorkers)
}

func goParseWorkers() int {
	return clampWorkers(scanParallelism(), minGoParseWorkers, maxGoParseWorkers)
}

// fileScanWorkers is the per-section worker count for a file pass over files
// inputs; see checkSupport.ParallelWorkers (one per usable CPU, at most 16,
// never more than there are files).
func fileScanWorkers(files int) int {
	return checkSupport.ParallelWorkers(files)
}
