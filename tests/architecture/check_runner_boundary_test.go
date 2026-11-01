// Package architecture_test enforces package-boundary invariants.
package architecture_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const (
	modulePath   = "github.com/devr-tools/codeguard"
	runnerPrefix = modulePath + "/internal/codeguard/runner"
)

var checksRoot = filepath.Join("..", "..", "internal", "codeguard", "checks")

// checkRunnerImportAllowlist lists the check files that still import a runner
// package, keyed by checks-relative path and then import path, with the reason
// each import has not been removed yet. Shrink it; do not grow it. Shared,
// side-effect-free helpers belong in checks/support or a leaf package under
// internal/codeguard (execlimit, cachefile, benchregression, ...), which both
// the runner and the checks may import.
var checkRunnerImportAllowlist = map[string]map[string]string{
	"quality/quality_ai_history.go": {
		runnerPrefix + "/support": "slop-score history persistence (SlopHistoryPathForBase, AppendSlopHistory, HistoryWriteDiagnostic) lives in runner/support; moving history recording out of the checks is a separate follow-up",
	},
	"performance/performance_score_history.go": {
		runnerPrefix + "/support": "performance-score history persistence lives in runner/support; same history-recording follow-up as quality_ai_history.go",
	},
	"agentcontext/legibility_history.go": {
		runnerPrefix + "/support": "legibility history persistence lives in runner/support; same history-recording follow-up as quality_ai_history.go",
	},
	"quality/quality_ai_corpus.go": {
		runnerPrefix + "/support": "unit-test fallback walk (WalkFilesWithOptions/FileWalkOptions) when env.ListTargetFiles is nil; the walk shares the runner's exclude and generated-path machinery, so it moves with that, not on its own",
	},
}

// TestChecksDoNotImportRunner keeps the dependency arrow pointing one way:
// the runner orchestrates the check sections, so a check importing a runner
// package couples a section to orchestration internals and risks cycles.
func TestChecksDoNotImportRunner(t *testing.T) {
	found := map[string]map[string]bool{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(checksRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		file, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			return parseErr
		}
		rel, relErr := filepath.Rel(checksRoot, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		for _, spec := range file.Imports {
			importPath, unquoteErr := strconv.Unquote(spec.Path.Value)
			if unquoteErr != nil {
				return unquoteErr
			}
			if importPath != runnerPrefix && !strings.HasPrefix(importPath, runnerPrefix+"/") {
				continue
			}
			if found[rel] == nil {
				found[rel] = map[string]bool{}
			}
			found[rel][importPath] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", checksRoot, err)
	}

	var violations []string
	for rel, imports := range found {
		for importPath := range imports {
			if _, allowed := checkRunnerImportAllowlist[rel][importPath]; !allowed {
				violations = append(violations, rel+" imports "+importPath)
			}
		}
	}
	sort.Strings(violations)
	for _, violation := range violations {
		t.Errorf("internal/codeguard/checks/%s; checks must not import runner packages - move the shared code into checks/support or a leaf package", violation)
	}

	var stale []string
	for rel, imports := range checkRunnerImportAllowlist {
		for importPath := range imports {
			if !found[rel][importPath] {
				stale = append(stale, rel+" -> "+importPath)
			}
		}
	}
	sort.Strings(stale)
	for _, entry := range stale {
		t.Errorf("allowlist entry %s is no longer needed; remove it from checkRunnerImportAllowlist", entry)
	}
}
