package quality

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// goAllocInLoopFindings flags allocation-heavy loop bodies: string += growth
// (including fmt.Sprintf accumulation) and appends to non-preallocated slices
// when the loop bound is knowable.
func goAllocInLoopFindings(env support.Context, file string, fset *token.FileSet, parsed *ast.File) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		growable := goGrowableSliceNames(fn.Body)
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			body := goLoopBody(node)
			if body == nil {
				return true
			}
			knowable := goLoopBoundKnowable(node)
			ast.Inspect(body, func(inner ast.Node) bool {
				assign, ok := inner.(*ast.AssignStmt)
				if !ok {
					return true
				}
				findings = append(findings, goAllocAssignFindings(env, file, fset, assign, growable, knowable)...)
				return true
			})
			return true
		})
	}
	return dedupeFindingsByLine(findings)
}

func goAllocAssignFindings(env support.Context, file string, fset *token.FileSet, assign *ast.AssignStmt, growable map[string]struct{}, knowableBound bool) []core.Finding {
	if message := goStringGrowthMessage(assign); message != "" {
		return []core.Finding{goAllocFinding(env, file, fset, assign, message)}
	}
	if !knowableBound {
		return nil
	}
	name, ok := goSelfAppendTarget(assign)
	if !ok {
		return nil
	}
	if _, candidate := growable[name]; !candidate {
		return nil
	}
	message := fmt.Sprintf("append to slice %q inside a loop with a knowable bound; preallocate capacity with make before the loop", name)
	return []core.Finding{goAllocFinding(env, file, fset, assign, message)}
}

func goAllocFinding(env support.Context, file string, fset *token.FileSet, assign *ast.AssignStmt, message string) core.Finding {
	pos := fset.Position(assign.Pos())
	return env.NewFinding(support.FindingInput{
		RuleID:  "quality.go.alloc-in-loop",
		Level:   "warn",
		Path:    file,
		Line:    pos.Line,
		Column:  pos.Column,
		Message: message,
	})
}

func goStringGrowthMessage(assign *ast.AssignStmt) string {
	if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return ""
	}
	target, ok := assign.Lhs[0].(*ast.Ident)
	if !ok {
		return ""
	}
	rhs := assign.Rhs[0]
	switch assign.Tok {
	case token.ADD_ASSIGN:
	case token.ASSIGN:
		binary, isBinary := rhs.(*ast.BinaryExpr)
		if !isBinary || binary.Op != token.ADD || !goExprMentionsIdent(binary, target.Name) {
			return ""
		}
	default:
		return ""
	}
	if !goExprLooksLikeString(rhs) {
		return ""
	}
	if goExprUsesSprintf(rhs) {
		return fmt.Sprintf("string %q accumulates fmt.Sprintf output inside a loop; use strings.Builder or fmt.Fprintf on a builder", target.Name)
	}
	return fmt.Sprintf("string %q grows by concatenation inside a loop; use strings.Builder", target.Name)
}

func goSelfAppendTarget(assign *ast.AssignStmt) (string, bool) {
	if assign.Tok != token.ASSIGN || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return "", false
	}
	target, ok := assign.Lhs[0].(*ast.Ident)
	if !ok {
		return "", false
	}
	call, ok := assign.Rhs[0].(*ast.CallExpr)
	if !ok || len(call.Args) < 2 {
		return "", false
	}
	fun, ok := call.Fun.(*ast.Ident)
	if !ok || fun.Name != "append" {
		return "", false
	}
	first, ok := call.Args[0].(*ast.Ident)
	if !ok || first.Name != target.Name {
		return "", false
	}
	return target.Name, true
}

func goExprLooksLikeString(expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(node ast.Node) bool {
		if lit, ok := node.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			found = true
		}
		return !found
	})
	return found || goExprUsesSprintf(expr)
}

func goExprUsesSprintf(expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		base, ok := sel.X.(*ast.Ident)
		if ok && base.Name == "fmt" && sel.Sel.Name == "Sprintf" {
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

func dedupeFindingsByLine(findings []core.Finding) []core.Finding {
	seen := make(map[string]struct{}, len(findings))
	out := make([]core.Finding, 0, len(findings))
	for _, finding := range findings {
		key := fmt.Sprintf("%s:%d:%s", finding.RuleID, finding.Line, finding.Message)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, finding)
	}
	return out
}
