package performance

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func goCorePerformanceFindings(env support.Context, file string, fset *token.FileSet, parsed *ast.File) []core.Finding {
	findings := make([]core.Finding, 0)
	rules := env.Config.Checks.PerformanceRules
	detectGoroutines := toggleEnabled(rules.DetectUnboundedConcurrency)
	detectSyncIO := toggleEnabled(rules.DetectSyncIOInHandlers)
	if !detectGoroutines && !detectSyncIO {
		return findings
	}
	httpAliases := importAliasesForPath(parsed, "net/http")
	syncIOAliases := syncIOAliases(parsed)

	walkASTWithStack(parsed, func(node ast.Node, stack []ast.Node) bool {
		switch node := node.(type) {
		case *ast.GoStmt:
			if detectGoroutines {
				if loop := nearestLoopAncestor(stack); loop != nil && !loopLaunchesBoundedWorkers(loop) {
					pos := fset.Position(node.Go)
					findings = append(findings, warnFinding(env, "performance.unbounded-goroutines-in-loop", file, pos.Line, pos.Column,
						"goroutine launched inside a loop should be bounded or queued explicitly"))
				}
			}
		case *ast.CallExpr:
			if !detectSyncIO {
				return true
			}
			fn := enclosingFunc(stack)
			if fn == nil || !isLikelyHTTPHandler(fn, httpAliases) || !isSyncIOCall(node, syncIOAliases) {
				return true
			}
			pos := fset.Position(node.Pos())
			findings = append(findings, warnFinding(env, "performance.sync-io-in-request-path", file, pos.Line, pos.Column,
				"synchronous file I/O in an HTTP request path can add tail latency"))
		}
		return true
	})

	return support.DedupeFindings(findings, func(finding core.Finding) string {
		return finding.RuleID + "|" + finding.Path + "|" + finding.Message + "|" + fmt.Sprintf("%d", finding.Line)
	})
}

func isLikelyHTTPHandler(fn *ast.FuncDecl, httpAliases map[string]struct{}) bool {
	if fn.Type == nil || fn.Type.Params == nil || len(httpAliases) == 0 {
		return false
	}
	if len(fn.Type.Params.List) != 2 {
		return false
	}
	firstType := normalizedExprString(fn.Type.Params.List[0].Type)
	secondType := normalizedExprString(fn.Type.Params.List[1].Type)
	for alias := range httpAliases {
		if firstType == alias+".ResponseWriter" && (secondType == "*"+alias+".Request" || secondType == alias+".Request") {
			return true
		}
	}
	return false
}

func isSyncIOCall(call *ast.CallExpr, aliases map[string]map[string]struct{}) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}
	operations, ok := aliases[ident.Name]
	if !ok {
		return false
	}
	_, ok = operations[selector.Sel.Name]
	return ok
}
