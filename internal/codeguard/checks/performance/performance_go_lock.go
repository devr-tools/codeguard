package performance

import (
	"go/ast"
	"go/token"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
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
	if scanStructuredStmt(s, stmt, held) {
		return
	}
	maybeReportBlockingCall(s, stmt, held)
	updateHeldLocks(held, stmt)
}
