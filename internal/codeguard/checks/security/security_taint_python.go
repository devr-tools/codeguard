package security

import (
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// pyTaintAnalyzer runs intra-file Python taint analysis with two summary
// passes followed by one reporting pass, resolving same-file call chains.
type pyTaintAnalyzer struct {
	taintEngine
	parsed     *support.ParsedFile
	models     pyModelBindings
	webRequest bool
}

func pythonTaintFindings(env support.Context, file string, source string) []core.Finding {
	parsed := support.ParsePython(source)
	analyzer := &pyTaintAnalyzer{
		taintEngine: newTaintEngine(env, file, taintRules{taintRuleID: "security.taint.python", ssrfRuleID: "security.ssrf.python", isSSRFSink: isPyHTTPSink}, false),
		parsed:      parsed,
		models:      newPyModelBindings(parsed.Imports),
		webRequest:  hasWebFrameworkImport(parsed.Imports),
	}
	analyzer.runSummaryPasses(analyzer.analyzeAll)
	analyzer.analyzeScope(analyzer.parsed.Module, true, true)
	return analyzer.findings
}

func hasWebFrameworkImport(imports []support.ParsedImport) bool {
	for _, imp := range imports {
		module := strings.ToLower(imp.Module)
		if strings.HasPrefix(module, "flask") || strings.HasPrefix(module, "django") || strings.HasPrefix(module, "fastapi") {
			return true
		}
	}
	return false
}

func (a *pyTaintAnalyzer) analyzeAll(emit bool, next map[string]*funcSummary) {
	for _, fn := range a.parsed.AllFunctions() {
		next[fn.Name] = a.analyzeScope(fn, emit, false)
	}
}

// analyzeScope walks one function or the module top level in order,
// tracking assignments and checking sinks.
func (a *pyTaintAnalyzer) analyzeScope(fn *support.ParsedFunction, emit bool, isModule bool) *funcSummary {
	scope := &pyScope{analyzer: a, requestModels: map[string]string{}}
	scope.stmtTaintScope = newStmtTaintScope(&a.taintEngine, emit, fn, scope)
	scope.bindParams(isModule)
	for _, statement := range fn.Statements {
		scope.processStatement(statement)
	}
	return scope.summary
}

type pyScope struct {
	stmtTaintScope
	analyzer      *pyTaintAnalyzer
	requestParam  bool
	requestModels map[string]string
}

// bindParams marks parameters as conditionally tainted. A leading self or
// cls receiver is skipped so call-site argument indexes line up.
func (s *pyScope) bindParams(isModule bool) {
	if isModule {
		return
	}
	index := 0
	for position, param := range s.fn.Params {
		if position == 0 && (param.Name == "self" || param.Name == "cls") {
			continue
		}
		if model := s.analyzer.models.requestModel(param); model != "" {
			s.requestParam = true
			s.requestModels[param.Name] = model
		}
		s.vars[param.Name] = &taintTrace{
			source:     "parameter " + param.Name,
			sourceLine: s.fn.StartLine,
			chain:      []string{param.Name},
			paramIndex: index,
		}
		index++
	}
}

func (s *pyScope) processStatement(statement support.ParsedStatement) {
	s.checkStatementSinks(statement)
	s.applyAssignments(statement)
	trimmed := strings.TrimSpace(statement.Text)
	if rest, isReturn := strings.CutPrefix(trimmed, "return "); isReturn {
		s.recordReturn(s.evalExpr(rest, statement.Line))
	}
}
