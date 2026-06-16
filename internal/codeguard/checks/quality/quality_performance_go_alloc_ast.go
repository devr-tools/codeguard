package quality

import (
	"go/ast"
	"go/token"
)

// goExprLooksLikeString walks only the additive structure of the expression:
// a string literal or fmt.Sprintf call must participate in the concatenation
// itself. Literals nested inside other call arguments carry no signal, since
// expressions such as depth += strings.Count(line, "{") accumulate integers.
func goExprLooksLikeString(expr ast.Expr) bool {
	switch value := expr.(type) {
	case *ast.BasicLit:
		return value.Kind == token.STRING
	case *ast.ParenExpr:
		return goExprLooksLikeString(value.X)
	case *ast.BinaryExpr:
		return value.Op == token.ADD && (goExprLooksLikeString(value.X) || goExprLooksLikeString(value.Y))
	case *ast.CallExpr:
		return goCallIsSprintf(value)
	default:
		return false
	}
}

func goCallIsSprintf(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	base, ok := sel.X.(*ast.Ident)
	return ok && base.Name == "fmt" && sel.Sel.Name == "Sprintf"
}

func goExprUsesSprintf(expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if ok && goCallIsSprintf(call) {
			found = true
		}
		return !found
	})
	return found
}

func goExprMentionsIdent(expr ast.Expr, name string) bool {
	found := false
	ast.Inspect(expr, func(node ast.Node) bool {
		if ident, ok := node.(*ast.Ident); ok && ident.Name == name {
			found = true
		}
		return !found
	})
	return found
}

// goGrowableSliceNames collects slice variables declared without preallocated
// capacity, such as var x []T, x := []T{}, or x := make([]T, 0).
func goGrowableSliceNames(body *ast.BlockStmt) map[string]struct{} {
	names := make(map[string]struct{})
	ast.Inspect(body, func(node ast.Node) bool {
		switch stmt := node.(type) {
		case *ast.DeclStmt:
			collectGrowableVarDecl(stmt, names)
		case *ast.AssignStmt:
			collectGrowableDefine(stmt, names)
		}
		return true
	})
	return names
}

func collectGrowableVarDecl(stmt *ast.DeclStmt, names map[string]struct{}) {
	decl, ok := stmt.Decl.(*ast.GenDecl)
	if !ok || decl.Tok != token.VAR {
		return
	}
	for _, spec := range decl.Specs {
		value, ok := spec.(*ast.ValueSpec)
		if !ok || len(value.Values) != 0 || !isSliceType(value.Type) {
			continue
		}
		for _, name := range value.Names {
			names[name.Name] = struct{}{}
		}
	}
}

func collectGrowableDefine(stmt *ast.AssignStmt, names map[string]struct{}) {
	if stmt.Tok != token.DEFINE {
		return
	}
	for idx, lhs := range stmt.Lhs {
		ident, ok := lhs.(*ast.Ident)
		if !ok || idx >= len(stmt.Rhs) {
			continue
		}
		if isGrowableSliceValue(stmt.Rhs[idx]) {
			names[ident.Name] = struct{}{}
		}
	}
}

func isGrowableSliceValue(expr ast.Expr) bool {
	switch value := expr.(type) {
	case *ast.CompositeLit:
		return isSliceType(value.Type) && len(value.Elts) == 0
	case *ast.CallExpr:
		fun, ok := value.Fun.(*ast.Ident)
		if !ok || fun.Name != "make" || len(value.Args) != 2 {
			return false
		}
		length, ok := value.Args[1].(*ast.BasicLit)
		return ok && isSliceType(value.Args[0]) && length.Value == "0"
	default:
		return false
	}
}

func isSliceType(expr ast.Expr) bool {
	arr, ok := expr.(*ast.ArrayType)
	return ok && arr.Len == nil
}

func goLoopBoundKnowable(node ast.Node) bool {
	switch loop := node.(type) {
	case *ast.RangeStmt:
		return true
	case *ast.ForStmt:
		cond, ok := loop.Cond.(*ast.BinaryExpr)
		if !ok {
			return false
		}
		switch cond.Op {
		case token.LSS, token.LEQ, token.GTR, token.GEQ:
			return goExprIsSimpleBound(cond.Y) || goExprIsSimpleBound(cond.X)
		default:
			return false
		}
	default:
		return false
	}
}

func goExprIsSimpleBound(expr ast.Expr) bool {
	switch bound := expr.(type) {
	case *ast.BasicLit:
		return bound.Kind == token.INT
	case *ast.Ident:
		return true
	case *ast.CallExpr:
		fun, ok := bound.Fun.(*ast.Ident)
		return ok && (fun.Name == "len" || fun.Name == "cap")
	default:
		return false
	}
}
