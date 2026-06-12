package quality

import (
	"fmt"
	"go/ast"
	"go/token"
	"regexp"
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

// --- TypeScript/JavaScript: lexical unreachable statements ---

var scriptTerminatorPattern = regexp.MustCompile(`^(?:return\b[^;{}]*;|throw\b[^;{}]*;|break\s*;|continue\s*;|return;?$|break$|continue$)`)
var scriptBlockResumePattern = regexp.MustCompile(`^(?:\}|case\b|default\s*:|else\b|catch\b|finally\b)`)

func scriptUnreachableFindings(env support.Context, file string, source string) []core.Finding {
	findings := make([]core.Finding, 0)
	sanitized := sanitizeScriptSource(source)
	depth := 0
	pendingDepth := -1
	for idx, line := range strings.Split(sanitized, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		startDepth := depth
		depth += strings.Count(line, "{") - strings.Count(line, "}")
		if pendingDepth >= 0 {
			if startDepth == pendingDepth && !scriptBlockResumePattern.MatchString(trimmed) {
				findings = append(findings, env.NewFinding(support.FindingInput{
					RuleID:  "quality.ai.dead-code",
					Level:   "warn",
					Path:    file,
					Line:    idx + 1,
					Column:  1,
					Message: "statement is unreachable because the previous statement unconditionally exits the block",
				}))
			}
			pendingDepth = -1
		}
		if scriptTerminatorPattern.MatchString(trimmed) && balancedParens(trimmed) {
			pendingDepth = depth
		}
	}
	return findings
}

func balancedParens(line string) bool {
	return strings.Count(line, "(") == strings.Count(line, ")")
}

// sanitizeScriptSource blanks out comment and string contents while keeping
// newlines so that brace tracking and line numbers stay accurate.
func sanitizeScriptSource(source string) string {
	out := []rune(source)
	const (
		codeState = iota
		lineComment
		blockComment
		singleQuote
		doubleQuote
		templateQuote
	)
	state := codeState
	for i := 0; i < len(out); i++ {
		ch := out[i]
		next := rune(0)
		if i+1 < len(out) {
			next = out[i+1]
		}
		switch state {
		case codeState:
			switch {
			case ch == '/' && next == '/':
				state = lineComment
				out[i] = ' '
			case ch == '/' && next == '*':
				state = blockComment
				out[i] = ' '
			case ch == '\'':
				state = singleQuote
			case ch == '"':
				state = doubleQuote
			case ch == '`':
				state = templateQuote
			}
		case lineComment:
			if ch == '\n' {
				state = codeState
			} else {
				out[i] = ' '
			}
		case blockComment:
			if ch == '*' && next == '/' {
				out[i] = ' '
				out[i+1] = ' '
				i++
				state = codeState
			} else if ch != '\n' {
				out[i] = ' '
			}
		case singleQuote, doubleQuote, templateQuote:
			closer := map[int]rune{singleQuote: '\'', doubleQuote: '"', templateQuote: '`'}[state]
			switch {
			case ch == '\\':
				out[i] = ' '
				if i+1 < len(out) && out[i+1] != '\n' {
					out[i+1] = ' '
					i++
				}
			case ch == closer:
				state = codeState
			case ch != '\n':
				out[i] = ' '
			}
		}
	}
	return string(out)
}

// --- TypeScript/JavaScript: unused file-local function declarations ---

var scriptLocalFunctionPattern = regexp.MustCompile(`(?m)^[ \t]*(?:async[ \t]+)?function[ \t]+([A-Za-z_$][\w$]*)[ \t]*\(`)

func scriptUnusedFunctionFindings(env support.Context, file string, source string) []core.Finding {
	sanitized := sanitizeScriptSource(source)
	findings := make([]core.Finding, 0)
	for _, match := range scriptLocalFunctionPattern.FindAllStringSubmatchIndex(sanitized, -1) {
		name := sanitized[match[2]:match[3]]
		lineStart := strings.LastIndexByte(sanitized[:match[0]], '\n') + 1
		declLine := sanitized[lineStart:lineEnd(sanitized, match[0])]
		if strings.Contains(declLine, "export") {
			continue
		}
		if countWordOccurrences(sanitized, name) > 1 {
			continue
		}
		line := 1 + strings.Count(sanitized[:match[2]], "\n")
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "quality.ai.dead-code",
			Level:   "warn",
			Path:    file,
			Line:    line,
			Column:  1,
			Message: fmt.Sprintf("file-local function %q is declared but never referenced in this file", name),
		}))
	}
	return findings
}

func lineEnd(source string, from int) int {
	if idx := strings.IndexByte(source[from:], '\n'); idx >= 0 {
		return from + idx
	}
	return len(source)
}

func countWordOccurrences(source string, word string) int {
	pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(word) + `\b`)
	return len(pattern.FindAllStringIndex(source, -1))
}

// --- Python: lexical unreachable statements ---

var pythonTerminatorPattern = regexp.MustCompile(`^(?:return\b|raise\b|break$|continue$|break\s|continue\s)`)
var pythonBlockResumePattern = regexp.MustCompile(`^(?:elif\b|else\s*:|except\b|finally\s*:|case\b)`)

func pythonDeadCodeFindings(env support.Context, file string, source string) []core.Finding {
	findings := make([]core.Finding, 0)
	pendingIndent := -1
	bracketDepth := 0
	continuation := false
	for idx, raw := range strings.Split(source, "\n") {
		line := stripPythonComment(raw)
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		logicalStart := bracketDepth == 0 && !continuation
		bracketDepth += strings.Count(line, "(") + strings.Count(line, "[") + strings.Count(line, "{")
		bracketDepth -= strings.Count(line, ")") + strings.Count(line, "]") + strings.Count(line, "}")
		if bracketDepth < 0 {
			bracketDepth = 0
		}
		continuation = strings.HasSuffix(trimmed, "\\")
		if !logicalStart {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if pendingIndent >= 0 {
			if indent == pendingIndent && !pythonBlockResumePattern.MatchString(trimmed) {
				findings = append(findings, env.NewFinding(support.FindingInput{
					RuleID:  "quality.ai.dead-code",
					Level:   "warn",
					Path:    file,
					Line:    idx + 1,
					Column:  1,
					Message: "statement is unreachable because the previous statement unconditionally exits the block",
				}))
			}
			pendingIndent = -1
		}
		if pythonTerminatorPattern.MatchString(trimmed) && bracketDepth == 0 && !continuation {
			pendingIndent = indent
		}
	}
	return findings
}

// stripPythonComment removes a trailing comment that starts outside string
// literals on the line. Multi-line strings are not tracked; the analysis is
// intentionally conservative.
func stripPythonComment(line string) string {
	inSingle := false
	inDouble := false
	for idx := 0; idx < len(line); idx++ {
		switch line[idx] {
		case '\\':
			idx++
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '#':
			if !inSingle && !inDouble {
				return line[:idx]
			}
		}
	}
	return line
}

// --- Python: unused private functions ---

var pythonPrivateFunctionPattern = regexp.MustCompile(`(?m)^[ \t]*def[ \t]+(_[A-Za-z0-9_]*)[ \t]*\(`)

func pythonUnusedPrivateFunctionFindings(env support.Context, file string, source string) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, match := range pythonPrivateFunctionPattern.FindAllStringSubmatchIndex(source, -1) {
		name := source[match[2]:match[3]]
		if strings.HasPrefix(name, "__") && strings.HasSuffix(name, "__") {
			continue
		}
		if countWordOccurrences(source, name) > 1 {
			continue
		}
		line := 1 + strings.Count(source[:match[2]], "\n")
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "quality.ai.dead-code",
			Level:   "warn",
			Path:    file,
			Line:    line,
			Column:  1,
			Message: fmt.Sprintf("private function %q is declared but never referenced in this file", name),
		}))
	}
	return findings
}
