package support

import (
	"go/ast"
	"go/token"
)

// CyclomaticComplexity returns the McCabe cyclomatic complexity of a Go
// function body, counted the way gocyclo counts it: 1, plus one for every
// if, for, and range statement, every non-default case of a switch, type
// switch, or select, and every && and || operator (each short-circuit
// operator is its own decision point). Function literals inside the body
// count toward the enclosing function.
func CyclomaticComplexity(body *ast.BlockStmt) int {
	if body == nil {
		return 0
	}
	complexity := 1
	ast.Inspect(body, func(n ast.Node) bool {
		if isDecisionPoint(n) {
			complexity++
		}
		return true
	})
	return complexity
}

func isDecisionPoint(n ast.Node) bool {
	switch node := n.(type) {
	case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt:
		return true
	case *ast.CaseClause:
		return node.List != nil // default: is not a decision point
	case *ast.CommClause:
		return node.Comm != nil // default: is not a decision point
	case *ast.BinaryExpr:
		return node.Op == token.LAND || node.Op == token.LOR
	}
	return false
}
