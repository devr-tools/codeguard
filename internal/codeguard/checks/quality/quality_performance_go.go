package quality

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"path"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var syncIOOperationsByImportPath = map[string]map[string]struct{}{
	"os": {
		"Create":    {},
		"Lstat":     {},
		"Open":      {},
		"OpenFile":  {},
		"ReadDir":   {},
		"ReadFile":  {},
		"Stat":      {},
		"WriteFile": {},
	},
	"io/ioutil": {
		"ReadDir":   {},
		"ReadFile":  {},
		"WriteFile": {},
	},
}

func goPerformanceFindings(env support.Context, file string, fset *token.FileSet, parsed *ast.File) []core.Finding {
	findings := make([]core.Finding, 0)
	httpAliases := importAliasesForPath(parsed, "net/http")
	syncIOAliases := syncIOAliases(parsed)

	stack := make([]ast.Node, 0, 32)
	ast.Inspect(parsed, func(n ast.Node) bool {
		if n == nil {
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			return false
		}

		stack = append(stack, n)
		switch node := n.(type) {
		case *ast.GoStmt:
			if hasLoopAncestor(stack[:len(stack)-1]) {
				pos := fset.Position(node.Go)
				findings = append(findings, env.NewFinding(support.FindingInput{
					RuleID:  "quality.unbounded-goroutines-in-loop",
					Level:   "warn",
					Path:    file,
					Line:    pos.Line,
					Column:  pos.Column,
					Message: "goroutine launched inside a loop should be bounded or queued explicitly",
				}))
			}
		case *ast.CallExpr:
			fn := enclosingFunc(stack[:len(stack)-1])
			if fn == nil || !isLikelyHTTPHandler(fn, httpAliases) || !isSyncIOCall(node, syncIOAliases) {
				return true
			}
			pos := fset.Position(node.Pos())
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID:  "quality.sync-io-in-request-path",
				Level:   "warn",
				Path:    file,
				Line:    pos.Line,
				Column:  pos.Column,
				Message: "synchronous file I/O in an HTTP request path can add tail latency",
			}))
		}
		return true
	})

	return support.DedupeFindings(findings, func(finding core.Finding) string {
		return finding.RuleID + "|" + finding.Path + "|" + finding.Message + "|" + fmt.Sprintf("%d", finding.Line)
	})
}

func hasLoopAncestor(stack []ast.Node) bool {
	for i := len(stack) - 1; i >= 0; i-- {
		switch stack[i].(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			return true
		}
	}
	return false
}

func enclosingFunc(stack []ast.Node) *ast.FuncDecl {
	for i := len(stack) - 1; i >= 0; i-- {
		if fn, ok := stack[i].(*ast.FuncDecl); ok {
			return fn
		}
	}
	return nil
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

func syncIOAliases(parsed *ast.File) map[string]map[string]struct{} {
	aliases := make(map[string]map[string]struct{})
	for _, imp := range parsed.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		operations, ok := syncIOOperationsByImportPath[importPath]
		if !ok {
			continue
		}
		alias := importLocalName(imp, importPath)
		if alias == "" {
			continue
		}
		aliases[alias] = operations
	}
	return aliases
}

func importAliasesForPath(parsed *ast.File, importPath string) map[string]struct{} {
	aliases := make(map[string]struct{})
	for _, imp := range parsed.Imports {
		if strings.Trim(imp.Path.Value, `"`) != importPath {
			continue
		}
		if alias := importLocalName(imp, importPath); alias != "" {
			aliases[alias] = struct{}{}
		}
	}
	return aliases
}

func importLocalName(imp *ast.ImportSpec, importPath string) string {
	if imp.Name != nil {
		switch imp.Name.Name {
		case "_", ".":
			return ""
		default:
			return imp.Name.Name
		}
	}
	return path.Base(importPath)
}

func normalizedExprString(expr ast.Expr) string {
	var buf bytes.Buffer
	_ = printer.Fprint(&buf, token.NewFileSet(), expr)
	return strings.ReplaceAll(buf.String(), " ", "")
}
