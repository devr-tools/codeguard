package support

import (
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type fileScanInput struct {
	sectionID string
	target    core.TargetConfig
	rel       string
	data      []byte
}

type fileScanSpec struct {
	sectionID string
	include   func(string) bool
	evaluator func(string, []byte) []core.Finding
	parallel  bool
}

const maxFileScanWorkers = 4

func ScanTargetFiles(sc Context, target core.TargetConfig, sectionID string, include func(string) bool, evaluator func(string, []byte) []core.Finding) []core.Finding {
	return scanTargetFiles(sc, target, fileScanSpec{
		sectionID: sectionID,
		include:   include,
		evaluator: evaluator,
		parallel:  true,
	})
}

func ScanTargetFilesSequential(sc Context, target core.TargetConfig, sectionID string, include func(string) bool, evaluator func(string, []byte) []core.Finding) []core.Finding {
	return scanTargetFiles(sc, target, fileScanSpec{
		sectionID: sectionID,
		include:   include,
		evaluator: evaluator,
		parallel:  false,
	})
}

func scanTargetFiles(sc Context, target core.TargetConfig, spec fileScanSpec) []core.Finding {
	selected := selectedTargetFiles(sc, target, spec.include)
	scanOne := func(file string) []core.Finding {
		data, err := sc.corpusRead(target.Path, file)
		if err != nil {
			return nil
		}
		return cachedFileFindings(sc, fileScanInput{
			sectionID: spec.sectionID,
			target:    target,
			rel:       file,
			data:      data,
		}, func() []core.Finding {
			return spec.evaluator(file, data)
		})
	}
	workers := 1
	if spec.parallel {
		workers = fileScanWorkers(len(selected))
	}
	if workers <= 1 {
		return scanTargetFilesSequentially(selected, scanOne)
	}
	return scanTargetFilesInParallel(selected, workers, scanOne)
}

func selectedTargetFiles(sc Context, target core.TargetConfig, include func(string) bool) []string {
	files, _ := sc.corpusFiles(target.Path)
	selected := make([]string, 0, len(files))
	for _, file := range files {
		if include(file) {
			selected = append(selected, file)
		}
	}
	return selected
}

func scanTargetFilesSequentially(selected []string, scanOne func(string) []core.Finding) []core.Finding {
	findings := make([]core.Finding, 0, len(selected))
	for _, file := range selected {
		findings = append(findings, scanOne(file)...)
	}
	return findings
}

func scanTargetFilesInParallel(selected []string, workers int, scanOne func(string) []core.Finding) []core.Finding {
	slots := make([][]core.Finding, len(selected))
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
			for {
				i := int(next.Add(1)) - 1
				if i >= len(selected) {
					return
				}
				slots[i] = scanOne(selected[i])
			}
		}()
	}
	wg.Wait()
	if panicked.Load() {
		panic(panicValue)
	}
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

func fileScanWorkers(files int) int {
	workers := runtime.NumCPU()
	if workers > maxFileScanWorkers {
		workers = maxFileScanWorkers
	}
	if workers > files {
		workers = files
	}
	if workers < 1 {
		workers = 1
	}
	return workers
}

func cachedFileFindings(sc Context, input fileScanInput, compute func() []core.Finding) []core.Finding {
	if sc.Cache == nil {
		return compute()
	}
	configHash := sc.sectionConfigHash(input.sectionID)
	key := cacheKey(input.sectionID, input.target.Path, input.rel)
	fileHash := hashBytes(input.data)

	sc.Cache.mu.Lock()
	entry, ok := sc.Cache.entries[key]
	sc.Cache.mu.Unlock()
	if ok && entry.FileHash == fileHash && entry.ConfigHash == configHash {
		return cloneFindings(entry.Findings)
	}

	findings := compute()

	sc.Cache.mu.Lock()
	sc.Cache.entries[key] = cacheEntry{
		FileHash:   fileHash,
		ConfigHash: configHash,
		Findings:   cloneFindings(findings),
	}
	sc.Cache.dirty = true
	sc.Cache.mu.Unlock()
	return findings
}
