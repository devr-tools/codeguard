package reliability

import (
	"fmt"
	"go/ast"
	"strings"
)

func importAliases(parsed *ast.File, importPath string) map[string]struct{} {
	aliases := map[string]struct{}{}
	for _, spec := range parsed.Imports {
		if strings.Trim(spec.Path.Value, "\"") != importPath {
			continue
		}
		if spec.Name != nil {
			aliases[spec.Name.Name] = struct{}{}
			continue
		}
		parts := strings.Split(importPath, "/")
		aliases[parts[len(parts)-1]] = struct{}{}
	}
	return aliases
}

func funcHasContextParam(fn *ast.FuncDecl) bool {
	if fn.Type == nil || fn.Type.Params == nil {
		return false
	}
	for _, field := range fn.Type.Params.List {
		if strings.Contains(exprString(field.Type), "context.Context") {
			return true
		}
	}
	return false
}

func isUnboundedHTTPCall(call *ast.CallExpr, aliases map[string]struct{}) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := selector.X.(*ast.Ident)
	if ok {
		if _, exists := aliases[ident.Name]; exists {
			switch selector.Sel.Name {
			case "Get", "Head", "Post", "PostForm":
				return true
			}
		}
	}
	return selector.Sel.Name == "Do"
}

func isBackgroundContextCall(call *ast.CallExpr) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := selector.X.(*ast.Ident)
	return ok && ident.Name == "context" && (selector.Sel.Name == "Background" || selector.Sel.Name == "TODO")
}

func isPanicCall(call *ast.CallExpr) bool {
	ident, ok := call.Fun.(*ast.Ident)
	return ok && ident.Name == "panic"
}

func isListenAndServeCall(call *ast.CallExpr, aliases map[string]struct{}) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	if selector.Sel.Name != "ListenAndServe" && selector.Sel.Name != "ListenAndServeTLS" {
		return false
	}
	if ident, ok := selector.X.(*ast.Ident); ok {
		_, exists := aliases[ident.Name]
		return exists || ident.Name == "http"
	}
	return true
}

func assignmentSwallowsCallError(assign *ast.AssignStmt) bool {
	hasBlank := false
	for _, lhs := range assign.Lhs {
		if ident, ok := lhs.(*ast.Ident); ok && ident.Name == "_" {
			hasBlank = true
		}
	}
	if !hasBlank {
		return false
	}
	for _, rhs := range assign.Rhs {
		if _, ok := rhs.(*ast.CallExpr); ok {
			return true
		}
	}
	return false
}

func assignedHTTPResponseVars(assign *ast.AssignStmt, aliases map[string]struct{}) []string {
	names := make([]string, 0, 1)
	for _, rhs := range assign.Rhs {
		call, ok := rhs.(*ast.CallExpr)
		if !ok || !isUnboundedHTTPCall(call, aliases) {
			continue
		}
		for _, lhs := range assign.Lhs {
			ident, ok := lhs.(*ast.Ident)
			if ok && ident.Name != "_" && strings.Contains(strings.ToLower(ident.Name), "resp") {
				names = append(names, ident.Name)
			}
		}
	}
	return names
}

func deferredCloseVars(block *ast.BlockStmt) map[string]struct{} {
	closed := map[string]struct{}{}
	ast.Inspect(block, func(node ast.Node) bool {
		deferStmt, ok := node.(*ast.DeferStmt)
		if !ok {
			return true
		}
		chain := selectorChain(deferStmt.Call)
		if chain == "" {
			return true
		}
		parts := strings.Split(chain, ".")
		if len(parts) >= 3 && parts[len(parts)-2] == "Body" && parts[len(parts)-1] == "Close" {
			closed[parts[0]] = struct{}{}
		}
		return true
	})
	return closed
}

func loopLooksLikeRetry(block *ast.BlockStmt) bool {
	return blockHasNameToken(block, "retry", "attempt", "backoff", "transient")
}

func blockHasBackoff(block *ast.BlockStmt) bool {
	return blockHasNameToken(block, "sleep", "after", "ticker", "backoff", "jitter")
}

func blockHasIdempotencyEvidence(block *ast.BlockStmt) bool {
	return blockHasNameToken(block, "idempot", "dedupe", "dedup", "once", "processed")
}

func blockHasNonIdempotentCall(block *ast.BlockStmt) bool {
	found := false
	ast.Inspect(block, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		name := strings.ToLower(callName(call))
		for _, token := range []string{"post", "put", "patch", "delete", "create", "update", "save", "insert", "publish", "send", "charge", "write"} {
			if strings.Contains(name, token) {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

func blockHasNameToken(block *ast.BlockStmt, tokens ...string) bool {
	found := false
	ast.Inspect(block, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.Ident:
			if nameHasToken(n.Name, tokens...) {
				found = true
				return false
			}
		case *ast.SelectorExpr:
			if nameHasToken(n.Sel.Name, tokens...) {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

func nameHasToken(name string, tokens ...string) bool {
	name = strings.ToLower(name)
	for _, token := range tokens {
		if strings.Contains(name, token) {
			return true
		}
	}
	return false
}

func fileHasSelector(parsed *ast.File, selector string) bool {
	found := false
	ast.Inspect(parsed, func(node ast.Node) bool {
		sel, ok := node.(*ast.SelectorExpr)
		if ok && sel.Sel.Name == selector {
			found = true
			return false
		}
		return true
	})
	return found
}

func isInsideLoop(root ast.Node, target ast.Node) bool {
	inside := false
	var stack []ast.Node
	ast.Inspect(root, func(node ast.Node) bool {
		if node == nil {
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			return true
		}
		if node == target {
			for _, item := range stack {
				switch item.(type) {
				case *ast.ForStmt, *ast.RangeStmt:
					inside = true
					return false
				}
			}
		}
		stack = append(stack, node)
		return true
	})
	return inside
}

func isErrorsNewCall(call *ast.CallExpr) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "New" {
		return false
	}
	ident, ok := selector.X.(*ast.Ident)
	return ok && ident.Name == "errors"
}

func callName(call *ast.CallExpr) string {
	return exprString(call.Fun)
}

func selectorChain(call *ast.CallExpr) string {
	if call == nil {
		return ""
	}
	return exprString(call.Fun)
}

func exprString(expr ast.Expr) string {
	switch n := expr.(type) {
	case *ast.Ident:
		return n.Name
	case *ast.SelectorExpr:
		left := exprString(n.X)
		if left == "" {
			return n.Sel.Name
		}
		return left + "." + n.Sel.Name
	case *ast.StarExpr:
		return "*" + exprString(n.X)
	case *ast.CallExpr:
		return exprString(n.Fun)
	default:
		return fmt.Sprintf("%T", expr)
	}
}
