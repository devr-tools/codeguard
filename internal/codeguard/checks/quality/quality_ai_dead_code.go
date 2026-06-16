package quality

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// --- Go: unreachable statements after unconditional terminators ---

func goUnreachableCodeFindings(env support.Context, file string, fset *token.FileSet, parsed *ast.File) []core.Finding {
	findings := make([]core.Finding, 0)
	flag := func(stmt ast.Stmt) {
		pos := fset.Position(stmt.Pos())
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "quality.ai.dead-code",
			Level:   "warn",
			Path:    file,
			Line:    pos.Line,
			Column:  pos.Column,
			Message: "statement is unreachable because the previous statement unconditionally exits the block",
		}))
	}
	ast.Inspect(parsed, func(node ast.Node) bool {
		switch block := node.(type) {
		case *ast.BlockStmt:
			inspectGoStatementList(block.List, flag)
		case *ast.CaseClause:
			inspectGoStatementList(block.Body, flag)
		case *ast.CommClause:
			inspectGoStatementList(block.Body, flag)
		}
		return true
	})
	return findings
}

func inspectGoStatementList(stmts []ast.Stmt, flag func(ast.Stmt)) {
	for idx, stmt := range stmts {
		if !goStatementTerminates(stmt) || idx+1 >= len(stmts) {
			continue
		}
		// Labeled statements after a terminator can still be reached via goto,
		// so only flag remainders that contain no labels.
		for _, rest := range stmts[idx+1:] {
			if _, ok := rest.(*ast.LabeledStmt); ok {
				return
			}
		}
		flag(stmts[idx+1])
		return
	}
}

func goStatementTerminates(stmt ast.Stmt) bool {
	switch typed := stmt.(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.BranchStmt:
		return typed.Tok == token.BREAK || typed.Tok == token.CONTINUE || typed.Tok == token.GOTO
	case *ast.ExprStmt:
		call, ok := typed.X.(*ast.CallExpr)
		if !ok {
			return false
		}
		ident, ok := call.Fun.(*ast.Ident)
		return ok && ident.Name == "panic"
	default:
		return false
	}
}

// --- Go: private package functions that are never referenced ---

type goParsedFile struct {
	rel    string
	fset   *token.FileSet
	parsed *ast.File
}

func goUnusedPrivateFunctionFindings(env support.Context, packageFiles []goParsedFile) []core.Finding {
	type declSite struct {
		rel  string
		pos  token.Position
		name string
	}
	declared := make([]declSite, 0)
	used := map[string]struct{}{}
	for _, file := range packageFiles {
		for _, decl := range file.parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && goFuncEligibleForUnusedCheck(file.rel, fn) {
				declared = append(declared, declSite{rel: file.rel, pos: file.fset.Position(fn.Name.Pos()), name: fn.Name.Name})
			}
		}
		collectGoUsedIdentifiers(file.parsed, used)
	}
	findings := make([]core.Finding, 0)
	for _, site := range declared {
		if _, ok := used[site.name]; ok {
			continue
		}
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "quality.ai.dead-code",
			Level:   "warn",
			Path:    site.rel,
			Line:    site.pos.Line,
			Column:  site.pos.Column,
			Message: fmt.Sprintf("private function %q is declared but never referenced within its package", site.name),
		}))
	}
	return findings
}

func goFuncEligibleForUnusedCheck(rel string, fn *ast.FuncDecl) bool {
	name := fn.Name.Name
	if fn.Recv != nil || name == "main" || name == "init" || name == "TestMain" {
		return false
	}
	if !startsLowercase(name) {
		return false
	}
	if strings.HasSuffix(rel, "_test.go") && hasGoTestEntrypointPrefix(name) {
		return false
	}
	// Compiler directives such as go:linkname or cgo exports imply external use.
	if fn.Doc != nil && strings.Contains(fn.Doc.Text(), "go:") {
		return false
	}
	return true
}

func hasGoTestEntrypointPrefix(name string) bool {
	for _, prefix := range []string{"Test", "Benchmark", "Fuzz", "Example"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func startsLowercase(name string) bool {
	if name == "" {
		return false
	}
	first := name[0]
	return first >= 'a' && first <= 'z'
}

// collectGoUsedIdentifiers records every identifier that appears outside the
// defining position of a function declaration name.
func collectGoUsedIdentifiers(parsed *ast.File, used map[string]struct{}) {
	declNamePos := map[token.Pos]struct{}{}
	for _, decl := range parsed.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			declNamePos[fn.Name.Pos()] = struct{}{}
		}
	}
	ast.Inspect(parsed, func(node ast.Node) bool {
		ident, ok := node.(*ast.Ident)
		if !ok {
			return true
		}
		if _, isDecl := declNamePos[ident.Pos()]; isDecl {
			return true
		}
		used[ident.Name] = struct{}{}
		return true
	})
}
