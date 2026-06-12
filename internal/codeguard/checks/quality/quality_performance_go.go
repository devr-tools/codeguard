package quality

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

func qualityToggleEnabled(value *bool) bool {
	return value == nil || *value
}

func goPerformanceFindings(env support.Context, file string, fset *token.FileSet, parsed *ast.File) []core.Finding {
	findings := make([]core.Finding, 0)
	if qualityToggleEnabled(env.Config.Checks.QualityRules.DetectNPlusOneQuery) {
		findings = append(findings, goNPlusOneFindings(env, file, fset, parsed)...)
	}
	if qualityToggleEnabled(env.Config.Checks.QualityRules.DetectAllocInLoop) {
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
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID:  "quality.n-plus-one-query",
				Level:   "warn",
				Path:    file,
				Line:    line,
				Column:  fset.Position(call.Pos()).Column,
				Message: fmt.Sprintf("query call %s inside a loop suggests an N+1 query pattern; batch the query or hoist it out of the loop", sel.Sel.Name),
			}))
			return true
		})
		return true
	})
	return findings
}
