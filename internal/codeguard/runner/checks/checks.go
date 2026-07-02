package checks

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"runtime"
	"sync"

	checkSupport "github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	govulncheckrunner "github.com/devr-tools/codeguard/internal/codeguard/runner/govulncheck"
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
			results[i] = safeRun(def.id, def.name, func() core.SectionResult {
				return def.run(ctx, sc, checkEnv)
			})
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

// safeRun executes one check section, recovering from any panic so that a single
// failing check (e.g. an index-out-of-range while parsing an untrusted file)
// degrades to a diagnostic warning for that section instead of aborting the
// entire scan. On panic it returns a section bearing a single warning finding
// and the remaining sections still run.
func safeRun(id string, name string, fn func() core.SectionResult) (result core.SectionResult) {
	defer func() {
		if r := recover(); r != nil {
			result = core.SectionResult{
				ID:     id,
				Name:   name,
				Status: core.StatusWarn,
				Findings: []core.Finding{{
					RuleID:  "checks.section.panic",
					Level:   "warning",
					Section: id,
					Message: fmt.Sprintf("%s check did not complete: internal error (%v)", name, r),
				}},
			}
		}
	}()
	return fn()
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
			})
		},
		FinalizeSection: func(id string, name string, findings []core.Finding) core.SectionResult {
			return runnersupport.FinalizeSection(sc, id, name, findings)
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
		IsSDKFacadeFile:      runnersupport.IsSDKFacadeFile,
		IsPromptFile: func(rel string) bool {
			return runnersupport.IsPromptFile(sc, rel)
		},
		RunGovulncheck: func(ctx context.Context, dir string, cmdName string) ([]core.Finding, error) {
			return govulncheckrunner.Run(ctx, dir, cmdName, sc)
		},
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
// behaves exactly as before this seam existed: every script rule takes its
// regex path. When enabled, parses are memoized per scan by the shared file
// corpus (one parse per file no matter how many sections query it).
func scriptFileParser(sc runnersupport.Context) func(string, []byte, checkSupport.ScriptLanguage) (*checkSupport.SyntaxTree, error) {
	if !sc.Cfg.Parsers.TreeSitterEnabled() {
		return nil
	}
	return func(path string, data []byte, lang checkSupport.ScriptLanguage) (*checkSupport.SyntaxTree, error) {
		return runnersupport.ParseScriptFile(sc, path, data, lang)
	}
}
