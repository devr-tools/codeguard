package security

import "go/ast"

func (s *goScope) walkStmts(stmts []ast.Stmt) {
	for _, stmt := range stmts {
		s.walkStmt(stmt)
	}
}

func (s *goScope) walkStmt(stmt ast.Stmt) {
	switch typed := stmt.(type) {
	case *ast.AssignStmt:
		s.handleAssign(typed)
	case *ast.DeclStmt:
		s.handleDecl(typed)
	case *ast.ExprStmt:
		s.evalExpr(typed.X)
	case *ast.ReturnStmt:
		s.handleReturn(typed)
	case *ast.IfStmt:
		s.walkIf(typed)
	case *ast.ForStmt:
		s.walkFor(typed)
	case *ast.RangeStmt:
		s.walkRange(typed)
	case *ast.BlockStmt:
		s.walkStmts(typed.List)
	default:
		s.walkOtherStmt(stmt)
	}
}

func (s *goScope) walkOtherStmt(stmt ast.Stmt) {
	switch typed := stmt.(type) {
	case *ast.SwitchStmt:
		if typed.Tag != nil {
			s.evalExpr(typed.Tag)
		}
		s.walkStmts(typed.Body.List)
	case *ast.CaseClause:
		s.walkStmts(typed.Body)
	case *ast.DeferStmt:
		s.evalExpr(typed.Call)
	case *ast.GoStmt:
		s.evalExpr(typed.Call)
	case *ast.LabeledStmt:
		s.walkStmt(typed.Stmt)
	}
}

func (s *goScope) walkIf(stmt *ast.IfStmt) {
	if stmt.Init != nil {
		s.walkStmt(stmt.Init)
	}
	s.evalExpr(stmt.Cond)
	s.walkStmts(stmt.Body.List)
	if stmt.Else != nil {
		s.walkStmt(stmt.Else)
	}
}

func (s *goScope) walkFor(stmt *ast.ForStmt) {
	if stmt.Init != nil {
		s.walkStmt(stmt.Init)
	}
	if stmt.Cond != nil {
		s.evalExpr(stmt.Cond)
	}
	s.walkStmts(stmt.Body.List)
}

// walkRange taints loop variables when ranging over a tainted collection.
func (s *goScope) walkRange(stmt *ast.RangeStmt) {
	taint := s.evalExpr(stmt.X)
	for _, target := range []ast.Expr{stmt.Key, stmt.Value} {
		ident, ok := target.(*ast.Ident)
		if !ok || ident.Name == "_" {
			continue
		}
		if taint != nil {
			s.vars[ident.Name] = taint.extended(ident.Name)
		} else {
			delete(s.vars, ident.Name)
		}
	}
	s.walkStmts(stmt.Body.List)
}
