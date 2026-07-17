package performance

import (
	"go/ast"
	"go/token"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
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

type goLockHeldConfig struct {
	syncIOAliases map[string]map[string]struct{}
	timeAliases   map[string]struct{}
	httpAliases   map[string]struct{}
	hasDatabase   bool
}

type goLockHeldScan struct {
	env      support.Context
	file     string
	fset     *token.FileSet
	cfg      goLockHeldConfig
	findings []core.Finding
	seen     map[int]struct{}
}

func goLockHeldFindings(env support.Context, file string, fset *token.FileSet, parsed *ast.File) []core.Finding {
	if !toggleEnabled(env.Config.Checks.PerformanceRules.DetectHotPathPatterns) {
		return nil
	}
	cfg := newGoLockHeldConfig(parsed)
	scan := goLockHeldScan{
		env:      env,
		file:     file,
		fset:     fset,
		cfg:      cfg,
		findings: make([]core.Finding, 0),
		seen:     make(map[int]struct{}),
	}
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		scan.scanBlock(fn.Body, map[string]struct{}{})
	}
	return scan.findings
}

func newGoLockHeldConfig(parsed *ast.File) goLockHeldConfig {
	_, hasDatabase := parsedImportPath(parsed, "database/sql")
	return goLockHeldConfig{
		syncIOAliases: syncIOAliases(parsed),
		timeAliases:   importAliasesForPath(parsed, "time"),
		httpAliases:   importAliasesForPath(parsed, "net/http"),
		hasDatabase:   hasDatabase,
	}
}

func (s *goLockHeldScan) scanBlock(block *ast.BlockStmt, held map[string]struct{}) {
	if block == nil {
		return
	}
	for _, stmt := range block.List {
		s.scanStmt(stmt, held)
	}
}

func (s *goLockHeldScan) scanStmt(stmt ast.Stmt, held map[string]struct{}) {
	switch node := stmt.(type) {
	case *ast.BlockStmt:
		s.scanBlock(node, cloneHeldLocks(held))
		return
	case *ast.IfStmt:
		s.maybeReportBlockingCall(node.Init, held)
		s.maybeReportBlockingExpr(node.Cond, held)
		s.scanBlock(node.Body, cloneHeldLocks(held))
		if node.Else != nil {
			s.scanStmt(node.Else, cloneHeldLocks(held))
		}
		return
	case *ast.ForStmt:
		s.maybeReportBlockingCall(node.Init, held)
		s.maybeReportBlockingExpr(node.Cond, held)
		s.maybeReportBlockingCall(node.Post, held)
		s.scanBlock(node.Body, cloneHeldLocks(held))
		return
	case *ast.RangeStmt:
		s.maybeReportBlockingExpr(node.X, held)
		s.scanBlock(node.Body, cloneHeldLocks(held))
		return
	case *ast.SwitchStmt:
		s.maybeReportBlockingCall(node.Init, held)
		s.maybeReportBlockingExpr(node.Tag, held)
		s.scanBlock(node.Body, cloneHeldLocks(held))
		return
	case *ast.TypeSwitchStmt:
		s.maybeReportBlockingCall(node.Init, held)
		s.maybeReportBlockingCall(node.Assign, held)
		s.scanBlock(node.Body, cloneHeldLocks(held))
		return
	case *ast.SelectStmt:
		s.scanBlock(node.Body, cloneHeldLocks(held))
		return
	}

	s.maybeReportBlockingCall(stmt, held)
	if key, kind, ok := mutexCallInStmt(stmt); ok {
		switch kind {
		case "lock", "rlock":
			held[key] = struct{}{}
		case "unlock", "runlock":
			delete(held, key)
		}
	}
}

func (s *goLockHeldScan) maybeReportBlockingExpr(expr ast.Expr, held map[string]struct{}) {
	if expr == nil {
		return
	}
	s.reportBlockingNodes(expr, held)
}

func (s *goLockHeldScan) maybeReportBlockingCall(node ast.Node, held map[string]struct{}) {
	if node == nil {
		return
	}
	s.reportBlockingNodes(node, held)
}

func (s *goLockHeldScan) reportBlockingNodes(root ast.Node, held map[string]struct{}) {
	if len(held) == 0 || root == nil {
		return
	}
	ast.Inspect(root, func(node ast.Node) bool {
		if _, ok := node.(*ast.FuncLit); ok {
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if blockingCallKind(s.cfg, call) == "" {
			return true
		}
		pos := s.fset.Position(call.Pos())
		if _, dup := s.seen[pos.Line]; dup {
			return false
		}
		s.seen[pos.Line] = struct{}{}
		s.findings = append(s.findings, warnFinding(s.env, "performance.go.lock-held-across-blocking-call", s.file, pos.Line, pos.Column,
			"mutex held across a blocking call can serialize callers and amplify tail latency; copy the needed state and release the lock before the call"))
		return false
	})
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
