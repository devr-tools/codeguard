package checks

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"

	checkSupport "github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

// Build runs every enabled section and returns their results in the fixed
// registry order. Independent sections run concurrently on a worker pool bounded
// by the CPU count (and each section additionally fans its file scans out on a
// small per-section pool, see runnersupport.ScanTargetFiles); the shared scan
// cache, artifact store, and file corpus are all concurrency-safe, and results
// are written into position-indexed slots so the output order is deterministic
// regardless of completion order.
func Build(ctx context.Context, sc runnersupport.Context) []core.SectionResult {
	sc = withSynchronizedSectionCallback(sc)
	checkEnv := buildCheckContext(ctx, sc)

	enabled := make([]sectionDef, 0, len(sectionRegistry))
	for _, def := range sectionRegistry {
		if def.enabled(sc) {
			enabled = append(enabled, def)
		}
	}

	results := make([]core.SectionResult, len(enabled))
	sem := make(chan struct{}, sectionConcurrency(len(enabled)))
	var wg sync.WaitGroup
	for i, def := range enabled {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			result, panicked := SafeRun(def.id, def.name, func() core.SectionResult {
				return def.run(ctx, sc, checkEnv)
			})
			if panicked {
				notifyRecoveredSection(sc, result)
			}
			results[i] = result
		}()
	}
	wg.Wait()
	return results
}

// sectionConcurrency bounds the number of sections running at once to the CPU
// count (and never more than there are sections to run).
func sectionConcurrency(sections int) int {
	if sections <= 1 {
		return 1
	}
	limit := runtime.NumCPU()
	if limit < 1 {
		limit = 1
	}
	if limit > sections {
		limit = sections
	}
	return limit
}

// withSynchronizedSectionCallback wraps the caller's OnSectionComplete streaming
// callback in a mutex so concurrent sections never invoke it simultaneously,
// preserving the single-threaded contract callers were written against.
func withSynchronizedSectionCallback(sc runnersupport.Context) runnersupport.Context {
	callback := sc.Opts.OnSectionComplete
	if callback == nil {
		return sc
	}
	var mu sync.Mutex
	sc.Opts.OnSectionComplete = func(section core.SectionResult) {
		mu.Lock()
		defer mu.Unlock()
		callback(section)
	}
	return sc
}

// SectionPanicDiagnosticID identifies the diagnostic a section reports when it
// crashed instead of completing.
const SectionPanicDiagnosticID = "scan.section.panic"

// sectionPanicStackFrames bounds the stack excerpt kept on the diagnostic: the
// frames nearest the panic locate the bug, and a full goroutine dump would
// swamp the JSON/SARIF report it travels in.
const sectionPanicStackFrames = 8

// SafeRun executes one check section, recovering from any panic so that a
// single crashing check (e.g. an index-out-of-range while parsing an untrusted
// file) does not abort the entire scan; the remaining sections still run.
//
// A crashed section has not checked anything, so it must never read as clean.
// The outcome is an operational fail-level core.Diagnostic and a failing
// section status for every section, not only gate-critical ones: which
// sections gate is the caller's policy, and a scanner that cannot say what it
// skipped cannot let any of them pass. It is a diagnostic rather than a finding
// on purpose: findings can be baselined, waived, or confidence-filtered, and a
// crash silenced that way would keep passing CI on every later run.
//
// panicked reports whether fn panicked; Build uses it to stream the synthetic
// result, since the section never reached FinalizeSection itself.
func SafeRun(id string, name string, fn func() core.SectionResult) (result core.SectionResult, panicked bool) {
	defer func() {
		if r := recover(); r != nil {
			result = sectionPanicResult(id, name, r, debug.Stack())
			panicked = true
		}
	}()
	return fn(), false
}

func sectionPanicResult(id string, name string, recovered any, stack []byte) core.SectionResult {
	return core.SectionResult{
		ID:       id,
		Name:     name,
		Status:   core.StatusFail,
		Findings: []core.Finding{},
		Diagnostics: []core.Diagnostic{{
			ID:          SectionPanicDiagnosticID,
			Level:       string(core.StatusFail),
			Kind:        "internal-error",
			Message:     fmt.Sprintf("%s check crashed and did not complete, so its results are missing; this is a codeguard bug, please report it with the diagnostic evidence", name),
			Operational: true,
			Evidence:    panicStackExcerpt(stack, sectionPanicStackFrames),
			Metadata: map[string]string{
				"section": id,
				"panic":   fmt.Sprint(recovered),
			},
		}},
	}
}

// panicStackExcerpt keeps the innermost frames below the panic as
// "function location" strings. debug.Stack() output is a goroutine header and
// then function-line/file-line pairs; everything up to and including the
// runtime panic frame is recovery machinery, not the bug.
func panicStackExcerpt(stack []byte, maxFrames int) []string {
	lines := strings.Split(strings.TrimSpace(string(stack)), "\n")
	if len(lines) > 0 && strings.HasPrefix(lines[0], "goroutine ") {
		lines = lines[1:]
	}
	start := 0
	for i := 0; i+1 < len(lines); i += 2 {
		if strings.HasPrefix(lines[i], "panic(") || strings.HasPrefix(lines[i], "runtime.gopanic(") {
			start = i + 2
		}
	}
	excerpt := make([]string, 0, maxFrames)
	for i := start; i+1 < len(lines) && len(excerpt) < maxFrames; i += 2 {
		excerpt = append(excerpt, strings.TrimSpace(lines[i])+" "+strings.TrimSpace(lines[i+1]))
	}
	return excerpt
}

// notifyRecoveredSection streams a crashed section's synthetic result to the
// OnSectionComplete callback, keeping its once-per-section contract. A panic in
// the callback is swallowed: re-raising it from a worker goroutine would crash
// the whole scan, which is what SafeRun exists to prevent.
func notifyRecoveredSection(sc runnersupport.Context, result core.SectionResult) {
	if sc.Opts.OnSectionComplete == nil {
		return
	}
	defer func() { _ = recover() }()
	sc.Opts.OnSectionComplete(result)
}

// contractsEnabled resolves the contracts toggle: an explicit config value
// wins, otherwise the family is enabled only for diff scans.
func contractsEnabled(sc runnersupport.Context) bool {
	if sc.Cfg.Checks.Contracts != nil {
		return *sc.Cfg.Checks.Contracts
	}
	return sc.Opts.Mode == core.ScanModeDiff
}

// contextEnabled resolves the agent-context toggle: an explicit config value
// wins, otherwise the family is enabled for full scans. Diff scans skip it by
// default because its signature findings are repo-level (missing agent docs,
// basename ambiguity) and would resurface on every PR regardless of the
// change under review.
func contextEnabled(sc runnersupport.Context) bool {
	if sc.Cfg.Checks.Context != nil {
		return *sc.Cfg.Checks.Context
	}
	return sc.Opts.Mode != core.ScanModeDiff
}

// buildCheckContext assembles the per-check callback surface. The scan ctx is
// captured by the git-backed callbacks (ListChangedFiles, ReadBaseFile), whose
// closure signatures carry no context of their own; they are only invoked
// while Build's sections run, so the scan ctx is the correct lifetime.
func buildCheckContext(ctx context.Context, sc runnersupport.Context) checkSupport.Context {
	// One resolver per scan: it memoizes a go.mod read and a directory listing
	// per target, and sections consult it concurrently.
	facades := runnersupport.NewSDKFacadeResolver(sc.Cfg)
	return checkSupport.Context{
		Config:    sc.Cfg,
		AIEnabled: sc.Opts.EnableAI || (sc.Cfg.AI.Enabled != nil && *sc.Cfg.AI.Enabled),
		Mode:      sc.Opts.Mode,
		BaseRef:   sc.Opts.BaseRef,
		DiffText:  sc.Opts.DiffText,
		ScanTime:  sc.Today,
		ListChangedFiles: func(target core.TargetConfig) ([]core.ChangedFile, error) {
			return runnersupport.ListChangedFiles(ctx, sc, target)
		},
		ReadBaseFile: func(target core.TargetConfig, rel string) ([]byte, error) {
			return runnersupport.ReadBaseFile(ctx, sc, target, rel)
		},
		ChangedFiles: runnersupport.ChangedDiffFiles(sc),
		VisitTargetFiles: func(target core.TargetConfig, include func(string) bool, visit func(rel string, data []byte)) {
			runnersupport.VisitTargetFiles(sc, target, include, visit)
		},
		ListTargetFiles: func(target core.TargetConfig) ([]string, error) {
			return runnersupport.ListTargetFiles(sc, target)
		},
		ReadTargetFile: func(target core.TargetConfig, rel string) ([]byte, error) {
			return runnersupport.ReadTargetFile(sc, target, rel)
		},
		DiffScope: func() map[string]core.ChangedLineRanges {
			out := make(map[string]core.ChangedLineRanges, len(sc.Diff))
			for path, ranges := range sc.Diff {
				out[path] = ranges.Export()
			}
			return out
		},
		ScanTargetFiles: func(target core.TargetConfig, sectionID string, include func(string) bool, evaluator func(string, []byte) []core.Finding) []core.Finding {
			return runnersupport.ScanTargetFiles(sc, target, sectionID, include, evaluator)
		},
		ScanTargetFilesWithDependencies: func(target core.TargetConfig, sectionID string, include func(string) bool, deps checkSupport.FileScanDependencies, evaluator func(string, []byte) []core.Finding) []core.Finding {
			return runnersupport.ScanTargetFilesWithDependencies(sc, target, sectionID, include, deps, evaluator)
		},
		ParseGoFile: func(path string, data []byte) (*token.FileSet, *ast.File, error) {
			return runnersupport.ParseGoFile(sc, path, data)
		},
		ParseScriptFile: scriptFileParser(sc),
		NewFinding: func(input checkSupport.FindingInput) core.Finding {
			return runnersupport.NewFinding(sc, runnersupport.FindingInput{
				RuleID:     input.RuleID,
				Level:      input.Level,
				Path:       input.Path,
				Line:       input.Line,
				Column:     input.Column,
				Message:    input.Message,
				Confidence: input.Confidence,
				Metadata:   input.Metadata,
			})
		},
		FinalizeSection: func(id string, name string, findings []core.Finding) core.SectionResult {
			return runnersupport.FinalizeSection(sc, id, name, findings)
		},
		FinalizeSectionWithDiagnostics: func(id string, name string, findings []core.Finding, diagnostics []core.Diagnostic) core.SectionResult {
			return runnersupport.FinalizeSectionWithDiagnostics(sc, id, name, findings, diagnostics)
		},
		PutArtifact: func(artifact core.Artifact) {
			sc.Artifacts.Put(artifact)
		},
		GetArtifact: func(id string) (core.Artifact, bool) {
			return sc.Artifacts.Get(id)
		},
		CountLines:           runnersupport.CountLines,
		CyclomaticComplexity: runnersupport.CyclomaticComplexity,
		TypeName:             runnersupport.TypeName,
		IsInternalOrCmdFile:  runnersupport.IsInternalOrCmdFile,
		IsCmdFile:            runnersupport.IsCmdFile,
		IsPublicPackageFile:  runnersupport.IsPublicPackageFile,
		SDKFacadeFor: func(target core.TargetConfig) checkSupport.SDKFacade {
			return facades.For(target)
		},
		IsPromptFile: func(rel string) bool {
			return runnersupport.IsPromptFile(sc, rel)
		},
		RunGovulncheck:         govulncheckCallback(sc),
		RunCPPFormat:           runCPPFormat,
		RunCPPSyntax:           runCPPSyntax,
		RunCommandCheck:        runnersupport.RunCommandCheck,
		RunCommandCheckWithEnv: runnersupport.RunCommandCheckWithEnv,
		RunDiffCommandCheck: func(ctx context.Context, dir string, baseRef string, check core.CommandCheckConfig) (string, error) {
			return runnersupport.RunDiffCommandCheckWithContext(ctx, sc, dir, baseRef, check)
		},
		NormalizedSeverity: runnersupport.NormalizedSeverity,
	}
}

// scriptFileParser wires the tree-sitter ParserProvider seam. The hook stays
// nil unless parsers.treesitter is "auto", so the default configuration
// behaves exactly as before this seam existed: every non-Go rule keeps its
// native fallback path. When enabled, parses are memoized per scan by the
// shared file corpus (one parse per file no matter how many sections query
// it).
func scriptFileParser(sc runnersupport.Context) func(string, []byte, checkSupport.ScriptLanguage) (*checkSupport.SyntaxTree, error) {
	if !sc.Cfg.Parsers.TreeSitterEnabled() {
		return nil
	}
	return func(path string, data []byte, lang checkSupport.ScriptLanguage) (*checkSupport.SyntaxTree, error) {
		return runnersupport.ParseScriptFile(sc, path, data, lang)
	}
}
