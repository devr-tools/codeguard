package quality

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
)

var conventionalMutationBoundaryPattern = regexp.MustCompile(`^(accept|apply|approve|archive|clear|close|commit|deliver|download|drop|ensure|exists|fetch|import|list|notify|open|process|read|reconcile|record|run|seed|submit|sync|toggle|upload)`)

var localAccumulatorExprPattern = regexp.MustCompile(`(?i)^(?:new\s+)?(?:array|formdata|map|object|set|urlsearchparams|weakmap|weakset)\b|^\[|^\{|^make\s*\(|^array\.from\b|\.map\s*\(|\.filter\s*\(|\.reduce\s*\(|^(?:bytes|strings)\.buffer\b|^strings\.builder\b`)

func localMutationTargets(fn precisionFunction) map[string]struct{} {
	params := paramNames(fn)
	targets := make(map[string]struct{})
	for _, assignment := range directAssignments(fn) {
		name := strings.TrimSpace(assignment.Name)
		if name == "" || assignment.Augmented {
			continue
		}
		if _, isParam := params[name]; isParam {
			continue
		}
		if assignmentLooksLocalAccumulator(fn, assignment) {
			targets[name] = struct{}{}
			continue
		}
		if fn.Returns && isAccumulatorLikeLocalName(name) && assignmentLooksLocalBuilder(fn, assignment) {
			targets[name] = struct{}{}
		}
	}
	if fn.Returns && isAccumulatorBuilderFunctionName(fn.Name) {
		for _, call := range directCalls(fn) {
			target := mutationCallTarget(call.Callee)
			if target == "" || !isAccumulatorLikeLocalName(target) {
				continue
			}
			if _, isParam := params[target]; isParam {
				continue
			}
			targets[target] = struct{}{}
		}
	}
	return targets
}

func directAssignments(fn precisionFunction) []support.ParsedAssignment {
	if len(fn.Nested) == 0 {
		return fn.Assignments
	}
	assignments := make([]support.ParsedAssignment, 0, len(fn.Assignments))
	for _, assignment := range fn.Assignments {
		if !callInNestedFunction(fn, assignment.Line) {
			assignments = append(assignments, assignment)
		}
	}
	return assignments
}

func directCalls(fn precisionFunction) []support.ParsedCall {
	if len(fn.Nested) == 0 {
		return fn.Calls
	}
	calls := make([]support.ParsedCall, 0, len(fn.Calls))
	for _, call := range fn.Calls {
		if !callInNestedFunction(fn, call.Line) {
			calls = append(calls, call)
		}
	}
	return calls
}

func directStatements(fn precisionFunction) []support.ParsedStatement {
	if len(fn.Nested) == 0 {
		return fn.Statements
	}
	statements := make([]support.ParsedStatement, 0, len(fn.Statements))
	for _, statement := range fn.Statements {
		if !callInNestedFunction(fn, statement.Line) {
			statements = append(statements, statement)
		}
	}
	return statements
}

func callInNestedFunction(fn precisionFunction, line int) bool {
	for _, nested := range fn.Nested {
		if line >= nested.Start && line <= nested.End {
			return true
		}
	}
	return false
}

func assignmentLooksLocalAccumulator(fn precisionFunction, assignment support.ParsedAssignment) bool {
	expr := strings.TrimSpace(assignment.Expr)
	if localAccumulatorExprPattern.MatchString(expr) {
		return true
	}
	statement := assignmentStatement(fn, assignment.Line)
	if statement == "" {
		return false
	}
	name := regexp.QuoteMeta(assignment.Name)
	return regexp.MustCompile(`(?i)\b(?:const|let|var)\s+`+name+`\b.*=\s*(?:new\s+)?(?:array|formdata|map|object|set|urlsearchparams|weakmap|weakset)\b`).MatchString(statement) ||
		regexp.MustCompile(`(?i)\bvar\s+`+name+`\s+(?:bytes\.buffer|strings\.builder)\b`).MatchString(statement) ||
		regexp.MustCompile(`(?i)\bstd::(?:vector|map|set|unordered_map|unordered_set|stringstream)\b[^;\n]*\b`+name+`\b`).MatchString(statement) ||
		regexp.MustCompile(`\b`+name+`\s*:=\s*(?:\[\]|\{\}|make\s*\(|(?:bytes|strings)\.Buffer\b|strings\.Builder\b)`).MatchString(statement) ||
		regexp.MustCompile(`(?i)\b(?:const|let|var)\s+`+name+`\b.*=\s*(?:\[|\{|array\.from\b|[^;\n]+\.map\s*\(|[^;\n]+\.filter\s*\(|new\s+urlsearchparams\b)`).MatchString(statement)
}

func assignmentLooksLocalBuilder(fn precisionFunction, assignment support.ParsedAssignment) bool {
	statement := strings.ToLower(assignmentStatement(fn, assignment.Line))
	expr := strings.ToLower(strings.TrimSpace(assignment.Expr))
	if statement == "" && expr == "" {
		return false
	}
	return strings.Contains(statement, "new ") ||
		strings.Contains(statement, "create") ||
		strings.Contains(statement, "build") ||
		strings.Contains(statement, "make") ||
		strings.Contains(expr, "new ") ||
		strings.Contains(expr, "create") ||
		strings.Contains(expr, "build") ||
		strings.Contains(expr, "make")
}

func isAccumulatorLikeLocalName(name string) bool {
	lowered := strings.ToLower(strings.Trim(name, "_$"))
	for _, token := range []string{
		"bucket", "buckets", "buffer", "builder", "calendar", "cells", "copy", "doc",
		"document", "filter", "filters", "form", "items", "lines", "params", "parts",
		"payload", "primarycells", "query", "result", "rows", "scopes", "sections",
		"serializer", "text", "urlparams", "values", "csv", "export", "map",
	} {
		if strings.Contains(lowered, token) {
			return true
		}
	}
	return false
}

func isAccumulatorBuilderFunctionName(name string) bool {
	lowered := strings.ToLower(strings.Trim(name, "_$"))
	for _, token := range []string{
		"bucket", "build", "collect", "derive", "format", "group", "map", "parse",
		"primary", "render", "serialize", "transform",
	} {
		if strings.Contains(lowered, token) {
			return true
		}
	}
	return false
}

func assignmentStatement(fn precisionFunction, line int) string {
	for _, statement := range fn.Statements {
		if statement.Line == line {
			if strings.TrimSpace(statement.Raw) != "" {
				return statement.Raw
			}
			return statement.Text
		}
	}
	return ""
}

func paramNames(fn precisionFunction) map[string]struct{} {
	params := make(map[string]struct{}, len(fn.Params))
	for _, param := range fn.Params {
		if param.Name != "" {
			params[param.Name] = struct{}{}
		}
	}
	return params
}

func isLocalMutationCall(callee string, localTargets map[string]struct{}) bool {
	if isBareLocalMutationCall(callee) {
		return true
	}
	target := mutationCallTarget(callee)
	if target == "" {
		return false
	}
	return isLocalMutationTarget(target, localTargets)
}

func isBareLocalMutationCall(callee string) bool {
	switch strings.TrimSpace(callee) {
	case "append", "Set", "Array", "Object", "Map", "WeakMap", "WeakSet", "push_back":
		return true
	default:
		return false
	}
}

func mutationCallTarget(callee string) string {
	callee = strings.TrimSpace(callee)
	if callee == "" {
		return ""
	}
	for _, sep := range []string{".", "->", "::"} {
		if idx := strings.Index(callee, sep); idx > 0 {
			return strings.TrimSpace(callee[:idx])
		}
	}
	return ""
}

func isLocalMutationTarget(name string, localTargets map[string]struct{}) bool {
	_, ok := localTargets[strings.TrimSpace(name)]
	return ok
}

func isBuilderAccumulatorMutationCall(fn precisionFunction, call support.ParsedCall) bool {
	if !fn.Returns || !isAccumulatorBuilderFunctionName(fn.Name) {
		return false
	}
	target := mutationCallTarget(call.Callee)
	if target == "" {
		return isBareLocalMutationCall(call.Callee)
	}
	params := paramNames(fn)
	if _, isParam := params[target]; isParam {
		return false
	}
	return isAccumulatorLikeLocalName(target)
}

func isBuilderAccumulatorAssignment(fn precisionFunction, assignment support.ParsedAssignment) bool {
	if !fn.Returns || !isAccumulatorBuilderFunctionName(fn.Name) {
		return false
	}
	params := paramNames(fn)
	if _, isParam := params[assignment.Name]; isParam {
		return false
	}
	return isAccumulatorLikeLocalName(assignment.Name)
}

func isFrameworkCommandBoundary(file string, name string) bool {
	if !isScriptLikeSourcePath(file) {
		return false
	}
	if !isHTTPMethodName(name) {
		return false
	}
	normalized := strings.ReplaceAll(file, "\\", "/")
	return strings.HasSuffix(normalized, "/route.ts") || strings.HasSuffix(normalized, "/route.tsx") ||
		strings.HasSuffix(normalized, "/route.js") || strings.HasSuffix(normalized, "/route.jsx")
}

func isHTTPMethodName(name string) bool {
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
		return true
	default:
		return false
	}
}

func isScriptEntrypoint(file string, name string) bool {
	if strings.TrimSpace(name) != "main" {
		return false
	}
	normalized := strings.ToLower(strings.ReplaceAll(file, "\\", "/"))
	base := filepath.Base(normalized)
	if strings.HasPrefix(base, "seed") || strings.HasPrefix(base, "backfill") || strings.HasPrefix(base, "import") || strings.HasPrefix(base, "cleanup") {
		return true
	}
	return strings.Contains(normalized, "/scripts/") ||
		strings.Contains(normalized, "/script/") ||
		strings.Contains(normalized, "/backfill") ||
		strings.Contains(normalized, "/seed") ||
		strings.Contains(normalized, "/import")
}
