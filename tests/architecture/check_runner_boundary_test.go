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

// TestChecksDoNotImportRunner keeps the dependency arrow pointing one way:
// the runner orchestrates the check sections, so a check importing a runner
// package couples a section to orchestration internals and risks cycles.
// There is no allowlist. Checks reach runner services only through the
// checks/support.Context hooks; shared, side-effect-free helpers belong in
// checks/support or a leaf package under internal/codeguard (execlimit,
// cachefile, benchregression, ...), which both the runner and the checks may
// import. Work with side effects that outlives a section, such as recording
// score history, belongs in the runner after the sections finalize.
func TestChecksDoNotImportRunner(t *testing.T) {
	var violations []string
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
			if importPath == runnerPrefix || strings.HasPrefix(importPath, runnerPrefix+"/") {
				violations = append(violations, rel+" imports "+importPath)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", checksRoot, err)
	}
	sort.Strings(violations)
	for _, violation := range violations {
		t.Errorf("internal/codeguard/checks/%s; checks must not import runner packages - use a checks/support.Context hook, or move the shared code into checks/support or a leaf package", violation)
	}
}
