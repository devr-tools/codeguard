package performance

import (
	"go/ast"
	"go/token"
)

func hasLoopAncestor(stack []ast.Node) bool {
	return nearestLoopAncestor(stack) != nil
}

func nearestLoopAncestor(stack []ast.Node) ast.Node {
	for i := len(stack) - 1; i >= 0; i-- {
		switch stack[i].(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			return stack[i]
		}
	}
	return nil
}

// loopLaunchesBoundedWorkers recognizes worker-pool construction, where a loop
// launching goroutines is bounded by design rather than data-driven.
func loopLaunchesBoundedWorkers(loop ast.Node) bool {
	switch node := loop.(type) {
	case *ast.RangeStmt:
		if node.Key == nil && node.Value == nil {
			return true
		}
		return bodyAcquiresSemaphore(node.Body)
	case *ast.ForStmt:
		if cond, ok := node.Cond.(*ast.BinaryExpr); ok && (cond.Op == token.LSS || cond.Op == token.LEQ) {
			if isFixedCountBound(cond.Y) {
				return true
			}
		}
		return bodyAcquiresSemaphore(node.Body)
	default:
		return false
	}
}

func isFixedCountBound(expr ast.Expr) bool {
	switch bound := expr.(type) {
	case *ast.BasicLit:
		return bound.Kind == token.INT
	case *ast.Ident:
		return true
	default:
		return false
	}
}

func bodyAcquiresSemaphore(body *ast.BlockStmt) bool {
	if body == nil {
		return false
	}
	acquired := false
	ast.Inspect(body, func(node ast.Node) bool {
		send, ok := node.(*ast.SendStmt)
		if !ok {
			return !acquired
		}
		if lit, isLit := send.Value.(*ast.CompositeLit); isLit {
			if structType, isStruct := lit.Type.(*ast.StructType); isStruct && len(structType.Fields.List) == 0 {
				acquired = true
			}
		}
		return !acquired
	})
	return acquired
}

func enclosingFunc(stack []ast.Node) *ast.FuncDecl {
	for i := len(stack) - 1; i >= 0; i-- {
		if fn, ok := stack[i].(*ast.FuncDecl); ok {
			return fn
		}
	}
	return nil
}
