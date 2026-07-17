package performance

import (
	"go/ast"
	"strings"
)

var (
	goBlockingHTTPNames = map[string]struct{}{
		"Get":      {},
		"Head":     {},
		"Post":     {},
		"PostForm": {},
	}
	goBlockingSQLMethodNames = map[string]struct{}{
		"Exec":            {},
		"ExecContext":     {},
		"Query":           {},
		"QueryContext":    {},
		"QueryRow":        {},
		"QueryRowContext": {},
	}
)

func updateHeldLocks(held map[string]struct{}, stmt ast.Stmt) {
	if key, kind, ok := mutexCallInStmt(stmt); ok {
		switch kind {
		case "lock", "rlock":
			held[key] = struct{}{}
		case "unlock", "runlock":
			delete(held, key)
		}
	}
}

func mutexCallInStmt(stmt ast.Stmt) (string, string, bool) {
	node, ok := stmt.(*ast.ExprStmt)
	if !ok {
		return "", "", false
	}
	call, _ := node.X.(*ast.CallExpr)
	if call == nil {
		return "", "", false
	}
	return mutexCallKind(call)
}

func mutexCallKind(call *ast.CallExpr) (string, string, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", "", false
	}
	switch sel.Sel.Name {
	case "Lock":
		return normalizedExprString(sel.X), "lock", true
	case "Unlock":
		return normalizedExprString(sel.X), "unlock", true
	case "RLock":
		return normalizedExprString(sel.X), "rlock", true
	case "RUnlock":
		return normalizedExprString(sel.X), "runlock", true
	default:
		return "", "", false
	}
}

func blockingCallKind(cfg goLockHeldConfig, call *ast.CallExpr) string {
	if _, _, ok := mutexCallKind(call); ok {
		return ""
	}
	if alias, name, ok := packageCall(call); ok {
		if operations, hit := cfg.syncIOAliases[alias]; hit {
			if _, op := operations[name]; op {
				return "sync-io"
			}
		}
		if aliasHas(cfg.timeAliases, alias) && name == "Sleep" {
			return "sleep"
		}
		if aliasHas(cfg.httpAliases, alias) && nameIn(goBlockingHTTPNames, name) {
			return "http"
		}
		return ""
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	if cfg.hasDatabase && nameIn(goBlockingSQLMethodNames, sel.Sel.Name) && looksLikeDatabaseReceiver(normalizedExprString(sel.X)) {
		return "sql"
	}
	if len(cfg.httpAliases) > 0 && sel.Sel.Name == "Do" && looksLikeHTTPClientReceiver(normalizedExprString(sel.X)) {
		return "http"
	}
	return ""
}

func looksLikeDatabaseReceiver(name string) bool {
	name = strings.ToLower(name)
	switch name {
	case "db", "tx", "stmt", "conn", "pool":
		return true
	}
	return strings.HasSuffix(name, ".db") ||
		strings.HasSuffix(name, ".tx") ||
		strings.HasSuffix(name, ".stmt") ||
		strings.HasSuffix(name, ".conn") ||
		strings.HasSuffix(name, ".pool")
}

func looksLikeHTTPClientReceiver(name string) bool {
	name = strings.ToLower(name)
	return strings.HasSuffix(name, "client") || strings.Contains(name, ".client")
}

func parsedImportPath(parsed *ast.File, importPath string) (*ast.ImportSpec, bool) {
	for _, imp := range parsed.Imports {
		if strings.Trim(imp.Path.Value, `"`) == importPath {
			return imp, true
		}
	}
	return nil, false
}

func cloneHeldLocks(src map[string]struct{}) map[string]struct{} {
	dst := make(map[string]struct{}, len(src))
	for key := range src {
		dst[key] = struct{}{}
	}
	return dst
}
