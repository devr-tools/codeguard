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

func isUnboundedHTTPCall(call *ast.CallExpr, aliases map[string]struct{}, boundedClients map[string]struct{}, boundedRequests map[string]struct{}) bool {
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
	if selector.Sel.Name != "Do" {
		return false
	}
	if len(call.Args) > 0 {
		if arg, ok := call.Args[0].(*ast.Ident); ok {
			if _, exists := boundedRequests[arg.Name]; exists {
				return false
			}
		}
	}
	if ident != nil {
		if _, exists := boundedClients[ident.Name]; exists {
			return false
		}
	}
	return true
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
		if !ok || !isHTTPResponseAcquisition(call, aliases) {
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

func boundedHTTPValues(block *ast.BlockStmt, aliases map[string]struct{}) (map[string]struct{}, map[string]struct{}) {
	clients := map[string]struct{}{}
	requests := map[string]struct{}{}
	ast.Inspect(block, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.AssignStmt:
			for idx, rhs := range n.Rhs {
				name := assignedNameAt(n.Lhs, idx)
				if name == "" {
					continue
				}
				if isHTTPClientWithTimeout(rhs, aliases) {
					clients[name] = struct{}{}
				}
				if isRequestWithContext(rhs) {
					requests[name] = struct{}{}
				}
			}
		case *ast.ValueSpec:
			for idx, rhs := range n.Values {
				if idx >= len(n.Names) {
					continue
				}
				name := n.Names[idx].Name
				if isHTTPClientWithTimeout(rhs, aliases) {
					clients[name] = struct{}{}
				}
				if isRequestWithContext(rhs) {
					requests[name] = struct{}{}
				}
			}
		}
		return true
	})
	return clients, requests
}

func assignedNameAt(lhs []ast.Expr, idx int) string {
	if len(lhs) == 0 {
		return ""
	}
	if idx >= len(lhs) {
		idx = len(lhs) - 1
	}
	ident, ok := lhs[idx].(*ast.Ident)
	if !ok || ident.Name == "_" {
		return ""
	}
	return ident.Name
}

func isHTTPClientWithTimeout(expr ast.Expr, aliases map[string]struct{}) bool {
	switch n := expr.(type) {
	case *ast.UnaryExpr:
		return isHTTPClientWithTimeout(n.X, aliases)
	case *ast.CompositeLit:
		if !isHTTPClientType(n.Type, aliases) {
			return false
		}
		for _, elt := range n.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "Timeout" {
				return true
			}
		}
	case *ast.CallExpr:
		return callName(n) == "http.Client" || strings.HasSuffix(callName(n), ".Client")
	}
	return false
}

func isHTTPClientType(expr ast.Expr, aliases map[string]struct{}) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Client" {
		return false
	}
	ident, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}
	if ident.Name == "http" {
		return true
	}
	_, exists := aliases[ident.Name]
	return exists
}

func isRequestWithContext(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	name := callName(call)
	return strings.HasSuffix(name, ".NewRequestWithContext") || strings.HasSuffix(name, ".WithContext")
}

func isHTTPResponseAcquisition(call *ast.CallExpr, aliases map[string]struct{}) bool {
	if isUnboundedHTTPCall(call, aliases, nil, nil) {
		return true
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	return selector.Sel.Name == "Do"
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
