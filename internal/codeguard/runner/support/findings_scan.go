package support

import (
	checkSupport "github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type fileScanInput struct {
	sectionID string
	target    core.TargetConfig
	rel       string
	data      []byte
	// depsHash digests the inputs beyond data that the evaluator reads; it is
	// empty for evaluators that depend on the file alone.
	depsHash string
}

type fileScanSpec struct {
	sectionID string
	include   func(string) bool
	evaluator func(string, []byte) fileScanResult
	parallel  bool
	// dependencies, when set, maps a file to the digest of the cross-file
	// inputs its findings depend on; see ScanTargetFilesWithDependencies.
	dependencies func(rel string) string
}

// fileScanResult is everything one file's evaluation produces. Diagnostics
// travel with the findings, through the cache, so a warm run reports the same
// diagnostics as a cold one.
type fileScanResult struct {
	findings    []core.Finding
	diagnostics []core.Diagnostic
}

func findingsOnly(evaluator func(string, []byte) []core.Finding) func(string, []byte) fileScanResult {
	return func(file string, data []byte) fileScanResult {
		return fileScanResult{findings: evaluator(file, data)}
	}
}

func ScanTargetFiles(sc Context, target core.TargetConfig, sectionID string, include func(string) bool, evaluator func(string, []byte) []core.Finding) []core.Finding {
	return scanTargetFiles(sc, target, fileScanSpec{
		sectionID: sectionID,
		include:   include,
		evaluator: findingsOnly(evaluator),
		parallel:  true,
	}).findings
}

// ScanTargetFilesWithDiagnostics is ScanTargetFiles for an evaluator that
// also reports per-file diagnostics. The diagnostics are stored in the same
// cache entry as the findings; collecting them as a side effect of the
// evaluator instead would lose them whenever a file is served from cache.
func ScanTargetFilesWithDiagnostics(sc Context, target core.TargetConfig, sectionID string, include func(string) bool, evaluator func(string, []byte) ([]core.Finding, []core.Diagnostic)) ([]core.Finding, []core.Diagnostic) {
	result := scanTargetFiles(sc, target, fileScanSpec{
		sectionID: sectionID,
		include:   include,
		evaluator: func(file string, data []byte) fileScanResult {
			findings, diagnostics := evaluator(file, data)
			return fileScanResult{findings: findings, diagnostics: diagnostics}
		},
		parallel: true,
	})
	return result.findings, result.diagnostics
}

func ScanTargetFilesSequential(sc Context, target core.TargetConfig, sectionID string, include func(string) bool, evaluator func(string, []byte) []core.Finding) []core.Finding {
	return scanTargetFiles(sc, target, fileScanSpec{
		sectionID: sectionID,
		include:   include,
		evaluator: findingsOnly(evaluator),
		parallel:  false,
	}).findings
}

func scanTargetFiles(sc Context, target core.TargetConfig, spec fileScanSpec) fileScanResult {
	selected := selectedTargetFiles(sc, target, spec.include)
	scanOne := func(file string) fileScanResult {
		data, err := sc.corpusRead(target.Path, file)
		if err != nil {
			sc.corpus.recordReadError(target.Path, file, err)
			return fileScanResult{}
		}
		// Binary content produces only noise: a line/regex detector run over
		// object code reports findings at offsets inside a compiled artifact.
		// Guarding here rather than per detector means no findings-producing
		// pass can skip the check. See LooksBinary for why this is not pushed
		// down to the corpus read or to VisitTargetFiles.
		if LooksBinary(data) {
			return fileScanResult{}
		}
		input := fileScanInput{
			sectionID: spec.sectionID,
			target:    target,
			rel:       file,
			data:      data,
		}
		if spec.dependencies != nil {
			input.depsHash = spec.dependencies(file)
		}
		return cachedFileResult(sc, input, func() fileScanResult {
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
	// A walk error is not dropped: the shared corpus records it as a failing
	// scan.target-walk diagnostic, which the section's finalize attaches. The
	// partial listing is still scanned.
	files, _ := sc.corpusFiles(target)
	selected := make([]string, 0, len(files))
	for _, file := range files {
		if include(file) {
			selected = append(selected, file)
		}
	}
	return selected
}

func scanTargetFilesSequentially(selected []string, scanOne func(string) fileScanResult) fileScanResult {
	slots := make([]fileScanResult, len(selected))
	for i, file := range selected {
		slots[i] = scanOne(file)
	}
	return mergeFileScanResults(slots)
}

// mergeFileScanResults concatenates per-file results in file order, so the
// output is deterministic regardless of which worker evaluated which file.
func mergeFileScanResults(slots []fileScanResult) fileScanResult {
	findingCount, diagnosticCount := 0, 0
	for _, slot := range slots {
		findingCount += len(slot.findings)
		diagnosticCount += len(slot.diagnostics)
	}
	merged := fileScanResult{findings: make([]core.Finding, 0, findingCount)}
	if diagnosticCount > 0 {
		merged.diagnostics = make([]core.Diagnostic, 0, diagnosticCount)
	}
	for _, slot := range slots {
		merged.findings = append(merged.findings, slot.findings...)
		merged.diagnostics = append(merged.diagnostics, slot.diagnostics...)
	}
	return merged
}

func scanTargetFilesInParallel(selected []string, workers int, scanOne func(string) fileScanResult) fileScanResult {
	slots := checkSupport.ParallelIndexed(len(selected), workers, func(i int) fileScanResult {
		return scanOne(selected[i])
	})
	return mergeFileScanResults(slots)
}

func cachedFileResult(sc Context, input fileScanInput, compute func() fileScanResult) fileScanResult {
	if sc.Cache == nil {
		return compute()
	}
	configHash := sc.sectionConfigHash(input.sectionID)
	key := cacheKey(input.sectionID, input.target.Path, input.rel)
	fileHash := hashBytes(input.data)

	sc.Cache.mu.Lock()
	entry, ok := sc.Cache.entries[key]
	sc.Cache.mu.Unlock()
	if ok && entry.FileHash == fileHash && entry.ConfigHash == configHash && entry.DepsHash == input.depsHash {
		return fileScanResult{findings: cloneFindings(entry.Findings), diagnostics: cloneDiagnostics(entry.Diagnostics)}
	}

	result := compute()

	sc.Cache.mu.Lock()
	sc.Cache.entries[key] = cacheEntry{
		FileHash:    fileHash,
		ConfigHash:  configHash,
		DepsHash:    input.depsHash,
		Findings:    cloneFindings(result.findings),
		Diagnostics: cloneDiagnostics(result.diagnostics),
	}
	sc.Cache.dirty = true
	sc.Cache.mu.Unlock()
	return result
}
