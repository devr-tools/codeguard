package quality

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
)

var conventionalMutationBoundaryPattern = regexp.MustCompile(`^(accept|apply|approve|archive|bulk|capture|clear|close|commit|copy|deliver|download|drop|ensure|exists|expose|fetch|fill|import|link|list|log|notify|open|process|read|reconcile|record|recompute|retire|run|seed|submit|sync|toggle|transfer|upload)`)

var localAccumulatorExprPattern = regexp.MustCompile(`(?i)^(?:new\s+)?(?:array|date|filereader|formdata|image|map|object|set|url|urlsearchparams|weakmap|weakset)\b|^\[|^\{|^make\s*\(|^array\.from\b|\.map\s*\(|\.filter\s*\(|\.reduce\s*\(|\.split\s*\(|cheerio\.load\s*\(|document\.createelement\s*\(|^(?:bytes|strings)\.buffer\b|^strings\.builder\b`)

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
		if assignmentLooksLocalScalarAccumulator(assignment, assignmentStatement(fn, assignment.Line)) {
			targets[name] = struct{}{}
			continue
		}
		if assignmentDerivedFromLocalMutationTarget(assignment, targets) {
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
	return regexp.MustCompile(`(?i)\b(?:const|let|var)\s+`+name+`\b.*=\s*(?:new\s+)?(?:array|date|filereader|formdata|image|map|object|set|url|urlsearchparams|weakmap|weakset)\b`).MatchString(statement) ||
		regexp.MustCompile(`(?i)\bvar\s+`+name+`\s+(?:bytes\.buffer|strings\.builder)\b`).MatchString(statement) ||
		regexp.MustCompile(`(?i)\bstd::(?:vector|map|set|unordered_map|unordered_set|stringstream)\b[^;\n]*\b`+name+`\b`).MatchString(statement) ||
		regexp.MustCompile(`\b`+name+`\s*:=\s*(?:\[\]|\{\}|make\s*\(|(?:bytes|strings)\.Buffer\b|strings\.Builder\b)`).MatchString(statement) ||
		regexp.MustCompile(`(?i)\b(?:const|let|var)\s+`+name+`\b.*=\s*(?:\[|\{|array\.from\b|[^;\n]+\.map\s*\(|[^;\n]+\.filter\s*\(|new\s+urlsearchparams\b)`).MatchString(statement) ||
		assignmentContinuationLooksLocalCollection(fn, assignment) ||
		assignmentLooksDateCloneOrHelper(assignment, statement)
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

func assignmentContinuationLooksLocalCollection(fn precisionFunction, assignment support.ParsedAssignment) bool {
	window := strings.ToLower(assignmentStatementWindow(fn, assignment.Line, 5))
	if window == "" {
		return false
	}
	name := strings.ToLower(regexp.QuoteMeta(assignment.Name))
	return regexp.MustCompile(`\b(?:const|let|var)\s+`+name+`\b`).MatchString(window) &&
		containsAny(window, []string{".map(", ".filter(", ".reduce(", "array.from(", "[...", ".slice("})
}

func assignmentLooksDateCloneOrHelper(assignment support.ParsedAssignment, statement string) bool {
	name := strings.ToLower(strings.Trim(assignment.Name, "_$"))
	if name == "" {
		return false
	}
	lowered := strings.ToLower(statement + " " + assignment.Expr)
	if !containsAny(lowered, []string{"date", "day", "week", "month", "time"}) {
		return false
	}
	return strings.Contains(lowered, "new date(") ||
		regexp.MustCompile(`=\s*(?:startof|endof|datefrom|today|yesterday)[A-Za-z0-9_$]*\s*\(`).MatchString(lowered) ||
		name == "x" && regexp.MustCompile(`=\s*[A-Za-z0-9_$]*day\s*\(`).MatchString(lowered)
}

func assignmentLooksLocalScalarAccumulator(assignment support.ParsedAssignment, statement string) bool {
	name := strings.ToLower(strings.Trim(assignment.Name, "_$"))
	if name == "" {
		return false
	}
	expr := strings.TrimSpace(assignment.Expr)
	lowered := strings.ToLower(statement + " " + expr)
	if !regexp.MustCompile(`(?i)\b(?:const|let|var)\s+` + regexp.QuoteMeta(assignment.Name) + `\b`).MatchString(statement) {
		return false
	}
	if !(regexp.MustCompile(`^-?\d+(?:\.\d+)?$`).MatchString(expr) ||
		regexp.MustCompile(`=\s*-?\d+(?:\.\d+)?\b`).MatchString(statement) ||
		strings.Contains(lowered, "score") ||
		strings.Contains(lowered, "total") ||
		strings.Contains(lowered, "count") ||
		strings.Contains(lowered, "sum") ||
		strings.Contains(lowered, "pct")) {
		return false
	}
	return len(name) <= 2 ||
		strings.Contains(name, "score") ||
		strings.Contains(name, "total") ||
		strings.Contains(name, "count") ||
		strings.Contains(name, "sum") ||
		strings.Contains(name, "pct")
}

func assignmentDerivedFromLocalMutationTarget(assignment support.ParsedAssignment, localTargets map[string]struct{}) bool {
	expr := strings.TrimSpace(assignment.Expr)
	if expr == "" || len(localTargets) == 0 {
		return false
	}
	for target := range localTargets {
		if target == "" {
			continue
		}
		if strings.HasPrefix(expr, target+".") || strings.HasPrefix(expr, target+"->") || strings.HasPrefix(expr, target+"::") {
			return true
		}
	}
	return false
}

func isAccumulatorLikeLocalName(name string) bool {
	lowered := strings.ToLower(strings.Trim(name, "_$"))
	for _, token := range []string{
		"bucket", "buckets", "buffer", "builder", "calendar", "cells", "copy", "doc",
		"canvas", "ctx", "cursor", "date", "document", "filter", "filters", "form", "img", "items", "lines", "params", "parts",
		"payload", "primarycells", "query", "result", "rows", "scopes", "sections",
		"serializer", "text", "urlparams", "values", "csv", "export", "map", "$",
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
		"primary", "render", "serialize", "transform", "clean", "filter",
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

func assignmentStatementWindow(fn precisionFunction, line int, lookahead int) string {
	parts := make([]string, 0, lookahead+1)
	for _, statement := range fn.Statements {
		if statement.Line < line || statement.Line > line+lookahead {
			continue
		}
		text := firstNonEmptyString(statement.Raw, statement.Text)
		if strings.TrimSpace(text) != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
}

func callStatementWindow(fn precisionFunction, line int, lookbehind int, lookahead int) string {
	parts := make([]string, 0, lookbehind+lookahead+1)
	for _, statement := range fn.Statements {
		if statement.Line < line-lookbehind || statement.Line > line+lookahead {
			continue
		}
		text := firstNonEmptyString(statement.Raw, statement.Text)
		if strings.TrimSpace(text) != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
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

func isLocalMutationCall(call support.ParsedCall, localTargets map[string]struct{}) bool {
	if isObjectAssignToLocalTarget(call, localTargets) {
		return true
	}
	return isLocalMutationCallee(call.Callee, localTargets)
}

func isLocalBuilderMutationCall(fn precisionFunction, call support.ParsedCall) bool {
	loweredCallee := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(call.Callee), " ", ""))
	if localDerivedCollectionMutationStatement(fn, call, loweredCallee) {
		return true
	}
	if strings.HasSuffix(loweredCallee, ".createelement") {
		statement := strings.ToLower(assignmentStatement(fn, call.Line))
		return strings.Contains(statement, "document.createelement(")
	}
	if loweredCallee == "createhmac" || loweredCallee == "createhash" {
		return true
	}
	if strings.Contains(loweredCallee, "createhmac") || strings.Contains(loweredCallee, "createhash") {
		statement := strings.ToLower(assignmentStatement(fn, call.Line))
		return strings.Contains(statement, "createhmac(") || strings.Contains(statement, "createhash(")
	}
	if loweredCallee == "replace" || strings.HasSuffix(loweredCallee, ".replace") {
		statement := strings.ToLower(assignmentStatement(fn, call.Line))
		if containsAny(statement, []string{"string(", ".replace(", ".trim(", ".tolowercase(", ".touppercase("}) {
			return true
		}
	}
	if loweredCallee != "update" && !strings.HasSuffix(loweredCallee, ".update") {
		return false
	}
	statement := strings.ToLower(assignmentStatement(fn, call.Line))
	return strings.Contains(statement, "createhmac(") || strings.Contains(statement, "createhash(")
}

func localDerivedCollectionMutationStatement(fn precisionFunction, call support.ParsedCall, loweredCallee string) bool {
	switch loweredCallee {
	case "pop", "sort", "reverse":
	default:
		if !strings.HasSuffix(loweredCallee, ".pop") && !strings.HasSuffix(loweredCallee, ".sort") && !strings.HasSuffix(loweredCallee, ".reverse") {
			return false
		}
	}
	statement := strings.ToLower(assignmentStatement(fn, call.Line))
	window := strings.ToLower(callStatementWindow(fn, call.Line, 4, 2))
	return containsAny(statement, []string{".split(", ".map(", ".filter(", ".reduce(", "array.from(", "[...", ".slice("}) ||
		containsAny(window, []string{".split(", ".map(", ".filter(", ".reduce(", "array.from(", "[...", ".slice("})
}

func isLocalMutationCallee(callee string, localTargets map[string]struct{}) bool {
	if isDerivedCollectionMutationCall(callee) {
		return true
	}
	if isBareLocalMutationCall(callee) {
		return true
	}
	target := mutationCallTarget(callee)
	if target == "" {
		return false
	}
	return isLocalMutationTarget(target, localTargets)
}

func isObjectAssignToLocalTarget(call support.ParsedCall, localTargets map[string]struct{}) bool {
	if !isObjectAssignCall(call) {
		return false
	}
	target := firstCallArgName(call)
	return target != "" && isLocalMutationTarget(target, localTargets)
}

func isObjectAssignCall(call support.ParsedCall) bool {
	return strings.EqualFold(strings.TrimSpace(call.Callee), "Object.assign")
}

func firstCallArgName(call support.ParsedCall) string {
	if len(call.Args) == 0 {
		return ""
	}
	arg := strings.TrimSpace(call.Args[0])
	if arg == "" {
		return ""
	}
	if idx := strings.IndexAny(arg, ".[("); idx > 0 {
		arg = strings.TrimSpace(arg[:idx])
	}
	if regexp.MustCompile(`^[A-Za-z_$][\w$]*$`).MatchString(arg) {
		return arg
	}
	return ""
}

func isDerivedCollectionMutationCall(callee string) bool {
	lowered := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(callee), " ", ""))
	return strings.Contains(lowered, ".split.") && (strings.HasSuffix(lowered, ".pop") || strings.HasSuffix(lowered, ".sort") || strings.HasSuffix(lowered, ".reverse"))
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
	if strings.HasPrefix(callee, "$.") {
		return "$"
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
	if isObjectAssignCall(call) {
		target = firstCallArgName(call)
	}
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

func isLocalScalarAccumulatorAssignment(fn precisionFunction, name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	if _, isParam := paramNames(fn)[name]; isParam {
		return false
	}
	quoted := regexp.QuoteMeta(name)
	declaration := regexp.MustCompile(`(?i)\b(?:const|let|var)\s+` + quoted + `\b[^;\n]*=\s*-?\d+(?:\.\d+)?\b`)
	for _, statement := range directStatements(fn) {
		if declaration.MatchString(firstNonEmptyString(statement.Raw, statement.Text)) {
			return true
		}
	}
	return false
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
