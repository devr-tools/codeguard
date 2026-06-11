package support

import "go/ast"

func CyclomaticComplexity(body *ast.BlockStmt) int {
	if body == nil {
		return 0
	}
	complexity := 1
	ast.Inspect(body, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.CaseClause, *ast.CommClause:
			complexity++
		}
		return true
	})
	return complexity
}
