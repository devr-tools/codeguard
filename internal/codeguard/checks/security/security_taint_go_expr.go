package security

import (
	"go/ast"
	"go/token"
)

// exprTypeText renders simple expression chains like "*http.Request",
// "os.Stdin", or "r.URL.Query().Get" for pattern matching.
func exprTypeText(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.StarExpr:
		return "*" + exprTypeText(typed.X)
	case *ast.SelectorExpr:
		return exprTypeText(typed.X) + "." + typed.Sel.Name
	case *ast.CallExpr:
		return exprTypeText(typed.Fun) + "()"
	case *ast.IndexExpr:
		return exprTypeText(typed.X) + "[]"
	case *ast.ParenExpr:
		return exprTypeText(typed.X)
	default:
		return ""
	}
}

func (s *goScope) sourceTaint(name string, pos token.Pos) *goTaint {
	return s.sourceTaintWithModel(name, pos, "")
}

func (s *goScope) sourceTaintWithModel(name string, pos token.Pos, model string) *goTaint {
	return &goTaint{
		source:     name,
		sourceLine: s.analyzer.line(pos),
		chain:      []string{name},
		paramIndex: -1,
		model:      model,
	}
}

// preferTaint picks a concrete source over a parameter-conditional taint.
func preferTaint(left *goTaint, right *goTaint) *goTaint {
	if left == nil {
		return right
	}
	if right == nil {
		return left
	}
	if left.paramIndex >= 0 && right.paramIndex < 0 {
		return right
	}
	return left
}

func (s *goScope) evalExpr(expr ast.Expr) *goTaint {
	switch typed := expr.(type) {
	case *ast.Ident:
		return s.vars[typed.Name]
	case *ast.CallExpr:
		return s.evalCall(typed)
	case *ast.BinaryExpr:
		return s.evalBinary(typed)
	case *ast.SelectorExpr:
		return s.evalSelector(typed)
	case *ast.IndexExpr:
		return s.evalIndex(typed)
	default:
		return s.evalOtherExpr(expr)
	}
}

func (s *goScope) evalOtherExpr(expr ast.Expr) *goTaint {
	switch typed := expr.(type) {
	case *ast.ParenExpr:
		return s.evalExpr(typed.X)
	case *ast.StarExpr:
		return s.evalExpr(typed.X)
	case *ast.UnaryExpr:
		return s.evalExpr(typed.X)
	case *ast.KeyValueExpr:
		return s.evalExpr(typed.Value)
	case *ast.CompositeLit:
		return s.evalComposite(typed)
	case *ast.FuncLit:
		s.walkStmts(typed.Body.List)
		return nil
	default:
		return nil
	}
}

func (s *goScope) evalBinary(expr *ast.BinaryExpr) *goTaint {
	left := s.evalExpr(expr.X)
	right := s.evalExpr(expr.Y)
	if expr.Op == token.ADD {
		return preferTaint(left, right)
	}
	return nil
}

func (s *goScope) evalComposite(lit *ast.CompositeLit) *goTaint {
	var taint *goTaint
	for _, element := range lit.Elts {
		taint = preferTaint(taint, s.evalExpr(element))
	}
	return taint
}

func (s *goScope) evalIndex(expr *ast.IndexExpr) *goTaint {
	if exprTypeText(expr.X) == "os.Args" {
		return s.sourceTaint("os.Args", expr.Pos())
	}
	s.evalExpr(expr.Index)
	return s.evalExpr(expr.X)
}

var goRequestFields = map[string]bool{"Body": true, "Header": true, "URL": true, "Form": true, "PostForm": true, "RequestURI": true}

func (s *goScope) evalSelector(expr *ast.SelectorExpr) *goTaint {
	if exprTypeText(expr) == "os.Args" {
		return s.sourceTaint("os.Args", expr.Pos())
	}
	if root, ok := expr.X.(*ast.Ident); ok && s.requestVars[root.Name] && goRequestFields[expr.Sel.Name] {
		return s.sourceTaint(root.Name+"."+expr.Sel.Name, expr.Pos())
	}
	return s.evalExpr(expr.X)
}

func rootIdent(expr ast.Expr) (string, bool) {
	for {
		switch typed := expr.(type) {
		case *ast.Ident:
			return typed.Name, true
		case *ast.SelectorExpr:
			expr = typed.X
		case *ast.CallExpr:
			expr = typed.Fun
		case *ast.ParenExpr:
			expr = typed.X
		default:
			return "", false
		}
	}
}
