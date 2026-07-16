package performance

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var goQueryMethodNames = map[string]struct{}{
	"Query":           {},
	"QueryRow":        {},
	"QueryContext":    {},
	"QueryRowContext": {},
	"Exec":            {},
	"ExecContext":     {},
}

// toggleEnabled treats a nil rule toggle as enabled, matching the rest of the
// rule pack defaults.
func toggleEnabled(value *bool) bool {
	return value == nil || *value
}

func goPerformanceFindings(env support.Context, file string, fset *token.FileSet, parsed *ast.File) []core.Finding {
	findings := goCorePerformanceFindings(env, file, fset, parsed)
	if toggleEnabled(env.Config.Checks.PerformanceRules.DetectNPlusOneQuery) {
		findings = append(findings, goNPlusOneFindings(env, file, fset, parsed)...)
	}
	if toggleEnabled(env.Config.Checks.PerformanceRules.DetectAllocInLoop) {
		findings = append(findings, goAllocInLoopFindings(env, file, fset, parsed)...)
	}
	return findings
}

func goLoopBody(node ast.Node) *ast.BlockStmt {
	switch loop := node.(type) {
	case *ast.ForStmt:
		return loop.Body
	case *ast.RangeStmt:
		return loop.Body
	default:
		return nil
	}
}

func goNPlusOneFindings(env support.Context, file string, fset *token.FileSet, parsed *ast.File) []core.Finding {
	findings := make([]core.Finding, 0)
	seen := make(map[int]struct{})
	ast.Inspect(parsed, func(node ast.Node) bool {
		body := goLoopBody(node)
		if body == nil {
			return true
		}
		ast.Inspect(body, func(inner ast.Node) bool {
			call, ok := inner.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if _, hit := goQueryMethodNames[sel.Sel.Name]; !hit {
				return true
			}
			line := fset.Position(call.Pos()).Line
			if _, dup := seen[line]; dup {
				return true
			}
			seen[line] = struct{}{}
			findings = append(findings, warnFinding(env, "performance.n-plus-one-query", file, line, fset.Position(call.Pos()).Column,
				fmt.Sprintf("query call %s inside a loop suggests an N+1 query pattern; batch the query or hoist it out of the loop", sel.Sel.Name)))
			return true
		})
		return true
	})
	return findings
}
