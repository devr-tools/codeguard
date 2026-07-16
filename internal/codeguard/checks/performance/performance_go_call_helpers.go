package performance

import "go/ast"

var regexCompileNames = map[string]struct{}{
	"Compile":          {},
	"MustCompile":      {},
	"CompilePOSIX":     {},
	"MustCompilePOSIX": {},
}

// A defer inside a func literal launched from a loop runs at the inner
// function boundary, so only loops found before crossing that boundary count.
func hasLoopAncestorWithinFunc(stack []ast.Node) bool {
	for i := len(stack) - 1; i >= 0; i-- {
		switch stack[i].(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			return true
		case *ast.FuncLit, *ast.FuncDecl:
			return false
		}
	}
	return false
}

func readerIsLimited(call *ast.CallExpr) bool {
	limited := false
	for _, arg := range call.Args {
		ast.Inspect(arg, func(node ast.Node) bool {
			switch value := node.(type) {
			case *ast.SelectorExpr:
				switch value.Sel.Name {
				case "LimitReader", "LimitedReader", "MaxBytesReader":
					limited = true
				}
			case *ast.Ident:
				switch value.Name {
				case "LimitReader", "LimitedReader", "MaxBytesReader":
					limited = true
				}
			}
			return !limited
		})
		if limited {
			break
		}
	}
	return limited
}

func packageCall(call *ast.CallExpr) (alias string, name string, ok bool) {
	sel, isSel := call.Fun.(*ast.SelectorExpr)
	if !isSel {
		return "", "", false
	}
	ident, isIdent := sel.X.(*ast.Ident)
	if !isIdent {
		return "", "", false
	}
	return ident.Name, sel.Sel.Name, true
}

func aliasHas(aliases map[string]struct{}, alias string) bool {
	_, ok := aliases[alias]
	return ok
}

func nameIn(names map[string]struct{}, name string) bool {
	_, ok := names[name]
	return ok
}
