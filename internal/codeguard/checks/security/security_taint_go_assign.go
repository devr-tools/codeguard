package security

import (
	"go/ast"
	"go/token"
	"strings"
)

func (s *goScope) handleReturn(stmt *ast.ReturnStmt) {
	for _, result := range stmt.Results {
		taint := s.evalExpr(result)
		if taint == nil {
			continue
		}
		if taint.paramIndex >= 0 {
			s.summary.paramsToReturn[taint.paramIndex] = true
		} else if s.summary.returnTaint == nil {
			s.summary.returnTaint = taint
		}
	}
}

func (s *goScope) handleAssign(stmt *ast.AssignStmt) {
	if len(stmt.Rhs) == 1 && len(stmt.Lhs) > 1 {
		taint := s.evalExpr(stmt.Rhs[0])
		s.markStdinReader(stmt.Lhs, stmt.Rhs[0])
		for _, lhs := range stmt.Lhs {
			s.assignTo(lhs, taint, stmt.Tok == token.DEFINE)
		}
		return
	}
	for idx, lhs := range stmt.Lhs {
		if idx >= len(stmt.Rhs) {
			break
		}
		taint := s.evalExpr(stmt.Rhs[idx])
		s.markStdinReader([]ast.Expr{lhs}, stmt.Rhs[idx])
		s.assignTo(lhs, taint, true)
	}
}

func (s *goScope) assignTo(lhs ast.Expr, taint *goTaint, allowClear bool) {
	ident, ok := lhs.(*ast.Ident)
	if !ok || ident.Name == "_" {
		return
	}
	if taint != nil {
		s.vars[ident.Name] = taint.extended(ident.Name)
		return
	}
	if allowClear {
		delete(s.vars, ident.Name)
	}
}

// markStdinReader tracks `reader := bufio.NewReader(os.Stdin)` and
// `tmpl := template.New(...)` style bindings so later method calls on them
// are recognized as sources and sinks respectively.
func (s *goScope) markStdinReader(lhs []ast.Expr, rhs ast.Expr) {
	call, ok := rhs.(*ast.CallExpr)
	if !ok {
		return
	}
	callee := exprTypeText(call.Fun)
	if strings.HasPrefix(callee, "template.") {
		s.markIdents(lhs, s.templateVars)
		return
	}
	if callee != "bufio.NewReader" && callee != "bufio.NewScanner" {
		return
	}
	if len(call.Args) != 1 || exprTypeText(call.Args[0]) != "os.Stdin" {
		return
	}
	s.markIdents(lhs, s.stdinReaders)
}

func (s *goScope) markIdents(lhs []ast.Expr, set map[string]bool) {
	for _, target := range lhs {
		if ident, ok := target.(*ast.Ident); ok && ident.Name != "_" {
			set[ident.Name] = true
		}
	}
}

func (s *goScope) handleDecl(stmt *ast.DeclStmt) {
	gen, ok := stmt.Decl.(*ast.GenDecl)
	if !ok {
		return
	}
	for _, spec := range gen.Specs {
		value, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for idx, name := range value.Names {
			if idx < len(value.Values) {
				s.assignTo(name, s.evalExpr(value.Values[idx]), true)
			}
		}
	}
}
