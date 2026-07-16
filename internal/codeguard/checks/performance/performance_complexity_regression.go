package performance

import (
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

const complexityRegressionRuleID = "performance.complexity-regression"

// complexityRegressionFindings warns when a change increases the maximum
// loop-nesting depth of an existing function relative to the diff base ref.
// It is a diff-only rule (like quality.coverage-delta): in full-scan mode, or
// when no diff scope is available, it stays silent. Coverage is Go-only in
// this first version; functions that do not exist at the base ref (new or
// renamed) are skipped, since there is no baseline to regress from.
func complexityRegressionFindings(env support.Context, target core.TargetConfig) []core.Finding {
	if !toggleEnabled(env.Config.Checks.PerformanceRules.DetectComplexityRegression) {
		return nil
	}
	if env.Mode != core.ScanModeDiff || env.DiffScope == nil || env.ReadBaseFile == nil {
		return nil
	}
	switch support.NormalizedLanguage(target.Language) {
	case "", "go":
	default:
		return nil
	}
	scope := env.DiffScope()
	if len(scope) == 0 {
		return nil
	}
	findings := make([]core.Finding, 0)
	for _, rel := range sortedChangedPaths(scope) {
		if !strings.HasSuffix(rel, ".go") {
			continue
		}
		findings = append(findings, complexityRegressionFileFindings(env, target, rel, scope[rel])...)
	}
	return findings
}

func complexityRegressionFileFindings(env support.Context, target core.TargetConfig, rel string, changed core.ChangedLineRanges) []core.Finding {
	headData, err := os.ReadFile(filepath.Join(target.Path, filepath.FromSlash(rel))) //nolint:gosec // target path from config + rel path from the scan's own git diff
	if err != nil {
		// Deleted (or unreadable) file: nothing on the head side to regress.
		return nil
	}
	headFset, headFile, err := support.ParseGoSource(env, rel, headData)
	if err != nil {
		// Unparseable Go is the quality section's problem (quality.parse-error).
		return nil
	}
	baseData, err := env.ReadBaseFile(target, rel)
	if err != nil {
		// Added file: every function is new, so there is no base to compare.
		return nil
	}
	baseDepths, err := baseFunctionLoopDepths(rel, baseData)
	if err != nil {
		return nil
	}
	scan := fileRegressionScan{fset: headFset, rel: rel, changed: changed, baseDepths: baseDepths}
	findings := make([]core.Finding, 0)
	for _, decl := range headFile.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		if finding, hit := scan.functionFinding(env, fn); hit {
			findings = append(findings, finding)
		}
	}
	return findings
}

// fileRegressionScan carries the per-file comparison state: the head-revision
// fileset, the file's changed line ranges, and the base-revision depths.
type fileRegressionScan struct {
	fset       *token.FileSet
	rel        string
	changed    core.ChangedLineRanges
	baseDepths map[string]int
}

// functionFinding compares one head-revision function against its
// base-revision depth and returns a finding when the maximum loop-nesting
// depth increased. Functions outside the changed line ranges or absent from
// the base revision report no finding.
func (s fileRegressionScan) functionFinding(env support.Context, fn *ast.FuncDecl) (core.Finding, bool) {
	start := s.fset.Position(fn.Pos()).Line
	end := s.fset.Position(fn.End()).Line
	if !changedRangesIntersect(s.changed, start, end) {
		return core.Finding{}, false
	}
	baseDepth, existsInBase := s.baseDepths[goFunctionKey(fn)]
	if !existsInBase {
		return core.Finding{}, false
	}
	headDepth, deepest := goMaxLoopNesting(s.fset, fn.Body)
	if headDepth <= baseDepth {
		return core.Finding{}, false
	}
	line := deepest
	if !s.changed.Contains(line) {
		// Anchor the finding to a changed line inside the function so it
		// survives diff-scope filtering even when the innermost loop line
		// itself predates this change (e.g. an existing loop was wrapped).
		line = firstChangedLineInSpan(s.changed, start, end)
	}
	return warnFinding(env, complexityRegressionRuleID, s.rel, line, 0,
		fmt.Sprintf("function %s: loop nesting depth increased from %d to %d in this change; verify the added iteration is not over unbounded data",
			goFunctionKey(fn), baseDepth, headDepth)), true
}
