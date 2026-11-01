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
	found, err := runnerImportsUnder(checksRoot)
	if err != nil {
		t.Fatalf("walk %s: %v", checksRoot, err)
	}
	violations := make([]string, 0, len(found))
	for rel, imports := range found {
		for importPath := range imports {
			violations = append(violations, rel+" imports "+importPath)
		}
	}
	sort.Strings(violations)
	for _, violation := range violations {
		t.Errorf("internal/codeguard/checks/%s; checks must not import runner packages - use a checks/support.Context hook, or move the shared code into checks/support or a leaf package", violation)
	}
}

// runnerImportsUnder maps each Go file under root (by slash-separated path
// relative to root) to the runner packages it imports.
func runnerImportsUnder(root string) (map[string]map[string]bool, error) {
	found := map[string]map[string]bool{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		imports, importErr := fileRunnerImports(fset, path)
		if importErr != nil {
			return importErr
		}
		if len(imports) > 0 {
			found[filepath.ToSlash(rel)] = imports
		}
		return nil
	})
	return found, err
}

// fileRunnerImports returns the runner packages one Go file imports.
func fileRunnerImports(fset *token.FileSet, path string) (map[string]bool, error) {
	file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	imports := map[string]bool{}
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return nil, err
		}
		if importPath == runnerPrefix || strings.HasPrefix(importPath, runnerPrefix+"/") {
			imports[importPath] = true
		}
	}
	return imports, nil
}
