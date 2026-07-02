package support

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type FindingInput struct {
	RuleID  string
	Level   string
	Path    string
	Line    int
	Column  int
	Message string
	Why     string
	// Confidence is "high", "medium", or "low"; empty means unspecified and is
	// treated as medium by consumers.
	Confidence string
}

type fileScanInput struct {
	sectionID string
	target    core.TargetConfig
	rel       string
	data      []byte
}

// maxFileScanWorkers caps how many files a single ScanTargetFiles call
// evaluates concurrently. Sections already run in parallel on a CPU-bounded
// pool (runner/checks.Build), so a small per-section cap keeps worst-case
// goroutine fan-out modest while still letting one large section spread its
// files across otherwise-idle cores instead of dominating wall clock.
const maxFileScanWorkers = 4

// ScanTargetFiles evaluates every included file under the target and returns
// the concatenated findings in walk order. Files are evaluated on a bounded
// per-call worker pool; the evaluator must therefore be safe to call from
// multiple goroutines (pure per-file evaluators are — see
// ScanTargetFilesSequential for the exceptions). Results are collected into
// position-indexed slots and flattened in file order, so the output is
// deterministic regardless of goroutine scheduling.
func ScanTargetFiles(sc Context, target core.TargetConfig, sectionID string, include func(string) bool, evaluator func(string, []byte) []core.Finding) []core.Finding {
	return scanTargetFiles(sc, target, sectionID, include, evaluator, true)
}

// ScanTargetFilesSequential is ScanTargetFiles without the per-file worker
// pool. It exists for evaluators that are not safe to run concurrently, such
// as ones that spawn a per-file subprocess (custom natural-language rules) and
// must not fan out. Evaluators that build cross-file state should use
// VisitTargetFiles instead, which also bypasses the findings cache.
func ScanTargetFilesSequential(sc Context, target core.TargetConfig, sectionID string, include func(string) bool, evaluator func(string, []byte) []core.Finding) []core.Finding {
	return scanTargetFiles(sc, target, sectionID, include, evaluator, false)
}

func scanTargetFiles(sc Context, target core.TargetConfig, sectionID string, include func(string) bool, evaluator func(string, []byte) []core.Finding, parallel bool) []core.Finding {
	files, _ := sc.corpusFiles(target.Path)
	selected := make([]string, 0, len(files))
	for _, file := range files {
		if include(file) {
			selected = append(selected, file)
		}
	}

	scanOne := func(file string) []core.Finding {
		data, err := sc.corpusRead(target.Path, file)
		if err != nil {
			return nil
		}
		return cachedFileFindings(sc, fileScanInput{
			sectionID: sectionID,
			target:    target,
			rel:       file,
			data:      data,
		}, func() []core.Finding {
			return evaluator(file, data)
		})
	}

	workers := 1
	if parallel {
		workers = fileScanWorkers(len(selected))
	}
	if workers <= 1 {
		findings := make([]core.Finding, 0, len(selected))
		for _, file := range selected {
			findings = append(findings, scanOne(file)...)
		}
		return findings
	}

	// Per-file findings land in position-indexed slots so the flattened output
	// preserves walk order exactly, independent of completion order.
	slots := make([][]core.Finding, len(selected))
	var next atomic.Int64
	// An evaluator panic on a worker goroutine must surface on the calling
	// goroutine, where safeRun (runner/checks) downgrades it to a section
	// warning; a raw goroutine panic would abort the whole process instead.
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

// fileScanWorkers bounds per-call file concurrency to min(maxFileScanWorkers,
// NumCPU, #files).
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

func NewFinding(sc Context, input FindingInput) core.Finding {
	normalizedPath := filepath.ToSlash(input.Path)
	meta := sc.RuleCatalog[input.RuleID]
	if input.Level == "" {
		input.Level = meta.DefaultLevel
	}
	input.Level = NormalizedSeverity(input.Level)
	sum := sha256.Sum256([]byte(strings.Join([]string{input.RuleID, normalizedPath, strconv.Itoa(input.Line), input.Message}, "|")))
	legacy := hex.EncodeToString(sum[:])
	contextFP := contextFingerprint(sc, input.RuleID, normalizedPath, input.Line)
	if contextFP == "" {
		contextFP = legacy
	}
	return core.Finding{
		RuleID:             input.RuleID,
		Level:              input.Level,
		Severity:           input.Level,
		Confidence:         core.NormalizedConfidence(input.Confidence),
		Title:              meta.Title,
		Section:            meta.Section,
		Message:            input.Message,
		Why:                firstNonEmpty(input.Why, input.Message),
		HowToFix:           meta.HowToFix,
		Path:               normalizedPath,
		Line:               input.Line,
		Column:             input.Column,
		Fingerprint:        legacy,
		ContextFingerprint: contextFP,
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func FinalizeSection(sc Context, id string, name string, findings []core.Finding) core.SectionResult {
	section := core.SectionResult{ID: id, Name: name, Status: core.StatusPass}
	active := make([]core.Finding, 0, len(findings))
	for _, finding := range findings {
		if sc.Opts.Mode == core.ScanModeDiff && finding.Path != "" && !matchesDiff(sc, finding) {
			continue
		}
		if suppressed, reason := IsSuppressed(sc, finding); suppressed {
			section.SuppressedCount++
			sc.RuleStats.RecordSuppressed(finding.RuleID, reason)
			continue
		}
		sc.RuleStats.RecordEmitted(finding.RuleID)
		active = append(active, finding)
		switch finding.Level {
		case "fail":
			section.Status = core.StatusFail
		case "warn":
			if section.Status != core.StatusFail {
				section.Status = core.StatusWarn
			}
		}
	}
	section.Findings = active
	if sc.Opts.OnSectionComplete != nil {
		sc.Opts.OnSectionComplete(section)
	}
	return section
}

func matchesDiff(sc Context, finding core.Finding) bool {
	scope, ok := sc.Diff[finding.Path]
	if !ok {
		return false
	}
	if scope.allChanged || finding.Line <= 0 {
		return true
	}
	for _, r := range scope.ranges {
		if finding.Line >= r[0] && finding.Line <= r[1] {
			return true
		}
	}
	return false
}

func IsPromptFile(sc Context, rel string) bool {
	rel = filepath.ToSlash(rel)
	ext := strings.ToLower(filepath.Ext(rel))
	for _, allowed := range sc.Cfg.Checks.PromptRules.FileExtensions {
		if strings.EqualFold(ext, allowed) {
			for _, token := range sc.Cfg.Checks.PromptRules.PathContains {
				if strings.Contains(strings.ToLower(rel), strings.ToLower(token)) {
					return true
				}
			}
		}
	}
	return false
}
