package quality

import (
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

const (
	namingGenericIdentifierRuleID          = "naming.generic-identifier"
	functionExcessiveParametersRuleID      = "function.excessive-parameters"
	functionMixedAbstractionLevelRuleID    = "function.mixed-abstraction-level"
	functionCommandQueryMixRuleID          = "function.command-query-mix"
	errorLoggedAndIgnoredRuleID            = "error.logged-and-ignored"
	errorContextLostRuleID                 = "error.context-lost"
	defensiveUncheckedTypeAssertionRuleID  = "defensive.unchecked-type-assertion"
	defensiveUnsafeNumericConversionRuleID = "defensive.unsafe-numeric-conversion"
	maintainabilityPublicSurfaceGrowthID   = "maintainability.public-surface-growth"
	maintainabilityDependencyGrowthID      = "maintainability.dependency-growth"
)

var (
	genericIdentifierNames = map[string]struct{}{
		"foo": {}, "bar": {}, "baz": {}, "qux": {},
		"tmp": {}, "temp": {}, "thing": {}, "stuff": {}, "obj": {}, "misc": {},
	}
	queryFunctionPrefixPattern = regexp.MustCompile(`^(get|find|list|load|read|lookup|fetch|is|has|can|should|compute|calculate|build|format|parse)`)
	mutatingCallPattern        = regexp.MustCompile(`(?i)(^|[.>:\-_])(add|append|assign|create|delete|emit|insert|mutate|persist|publish|remove|save|send|set|store|update|upsert|write)([A-Z_:\-.]|$)`)
	lowLevelOperationPattern   = regexp.MustCompile(`(?i)(\bsql\.|\.query\(|\.exec\(|\bhttp\.|\bfetch\(|\baxios\.|\brequests\.|\bjson\.|\bJSON\.|\bos\.Getenv\b|\bprocess\.env\b|\bfs\.|#include\b)`)
	goPublicDeclPattern        = regexp.MustCompile(`(?m)^(?:func|type|var|const)\s+(?:\([^)]*\)\s*)?([A-Z][A-Za-z0-9_]*)\b`)
	pythonPublicDeclPattern    = regexp.MustCompile(`(?m)^class\s+([A-Za-z]\w*)\b|^def\s+([A-Za-z]\w*)\s*\(`)
	tsPublicDeclPattern        = regexp.MustCompile(`(?m)^export\s+(?:declare\s+)?(?:async\s+)?(?:class|interface|type|enum|function|const|let|var)\s+([A-Za-z_$][\w$]*)\b`)
	cppPublicDeclPattern       = regexp.MustCompile(`(?m)^\s*(?:class|struct)\s+([A-Z]\w*)\b|^\s*(?:[A-Za-z_][\w:<>,\s*&~]*\s+)+([A-Z]\w*)\s*\([^;{}]*\)\s*;`)
	cppIncludePattern          = regexp.MustCompile(`(?m)^\s*#include\s+[<"]([^>"]+)[>"]`)
)

type precisionFunction struct {
	Name        string
	StartLine   int
	EndLine     int
	Signature   string
	Params      []support.ParsedParam
	Assignments []support.ParsedAssignment
	Calls       []support.ParsedCall
	Statements  []support.ParsedStatement
	Body        string
	Returns     bool
}

func precisionWarnFinding(env support.Context, ruleID string, file string, line int, message string, confidence string) core.Finding {
	return env.NewFinding(support.FindingInput{
		RuleID:     ruleID,
		Level:      "warn",
		Path:       file,
		Line:       line,
		Column:     1,
		Message:    message,
		Confidence: confidence,
	})
}

func localPrecisionEnabled(env support.Context) bool {
	return env.Config.Checks.QualityRules.LocalPrecision == nil || *env.Config.Checks.QualityRules.LocalPrecision
}

func excessiveParameterFinding(env support.Context, file string, fn functionMetrics) []core.Finding {
	if fn.Params <= env.Config.Checks.QualityRules.MaxParameters {
		return nil
	}
	return []core.Finding{precisionWarnFinding(env, functionExcessiveParametersRuleID, file, fn.StartLine,
		fmt.Sprintf("function %s has %d parameters; prefer grouping related inputs or splitting responsibilities", fn.Name, fn.Params),
		core.ConfidenceHigh)}
}

func goPrecisionFindings(env support.Context, file string, fset *token.FileSet, parsed *ast.File, data []byte) []core.Finding {
	findings := make([]core.Finding, 0)
	ast.Inspect(parsed, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			fn := goPrecisionFunction(fset, node, data)
			findings = append(findings, precisionFunctionFindings(env, file, fn)...)
			if node.Body != nil {
				findings = append(findings, goDefensiveFindings(env, file, fset, node.Body)...)
			}
		case *ast.GenDecl:
			findings = append(findings, goGenericDeclFindings(env, file, fset, node)...)
		}
		return true
	})
	return findings
}

func goPrecisionFunction(fset *token.FileSet, fn *ast.FuncDecl, data []byte) precisionFunction {
	out := precisionFunction{
		Name:      fn.Name.Name,
		StartLine: fset.Position(fn.Pos()).Line,
		EndLine:   fset.Position(fn.End()).Line,
		Params:    goParsedParams(fn),
		Returns:   goFuncReturnsValue(fn),
	}
	if fn.Body == nil {
		return out
	}
	start := fset.Position(fn.Body.Lbrace).Offset
	end := fset.Position(fn.Body.Rbrace).Offset
	if start >= 0 && end > start && end <= len(data) {
		out.Body = string(data[start:end])
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			out.Assignments = append(out.Assignments, goAssignments(fset, node)...)
		case *ast.ValueSpec:
			for _, name := range node.Names {
				out.Assignments = append(out.Assignments, support.ParsedAssignment{Name: name.Name, Line: fset.Position(name.Pos()).Line})
			}
		case *ast.CallExpr:
			out.Calls = append(out.Calls, support.ParsedCall{Callee: goCallName(node.Fun), Line: fset.Position(node.Pos()).Line})
		case *ast.ReturnStmt:
			out.Returns = out.Returns || len(node.Results) > 0
		}
		return true
	})
	for idx, line := range strings.Split(out.Body, "\n") {
		if strings.TrimSpace(line) != "" {
			out.Statements = append(out.Statements, support.ParsedStatement{Line: fset.Position(fn.Body.Lbrace).Line + idx, Text: line, Raw: line})
		}
	}
	return out
}

func goParsedParams(fn *ast.FuncDecl) []support.ParsedParam {
	if fn.Type == nil || fn.Type.Params == nil {
		return nil
	}
	params := make([]support.ParsedParam, 0)
	for _, field := range fn.Type.Params.List {
		typ := ""
		if field.Type != nil {
			typ = fmt.Sprintf("%T", field.Type)
		}
		for _, name := range field.Names {
			params = append(params, support.ParsedParam{Name: name.Name, Type: typ})
		}
		if len(field.Names) == 0 {
			params = append(params, support.ParsedParam{Type: typ})
		}
	}
	return params
}

func goFuncReturnsValue(fn *ast.FuncDecl) bool {
	return fn.Type != nil && fn.Type.Results != nil && len(fn.Type.Results.List) > 0
}

func goAssignments(fset *token.FileSet, stmt *ast.AssignStmt) []support.ParsedAssignment {
	assignments := make([]support.ParsedAssignment, 0, len(stmt.Lhs))
	for _, expr := range stmt.Lhs {
		if ident, ok := expr.(*ast.Ident); ok {
			assignments = append(assignments, support.ParsedAssignment{Name: ident.Name, Line: fset.Position(ident.Pos()).Line})
		}
	}
	return assignments
}

func goCallName(expr ast.Expr) string {
	switch value := expr.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.SelectorExpr:
		prefix := goCallName(value.X)
		if prefix == "" {
			return value.Sel.Name
		}
		return prefix + "." + value.Sel.Name
	default:
		return ""
	}
}

func goGenericDeclFindings(env support.Context, file string, fset *token.FileSet, decl *ast.GenDecl) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, spec := range decl.Specs {
		value, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for _, name := range value.Names {
			if isGenericIdentifier(name.Name) {
				findings = append(findings, precisionWarnFinding(env, namingGenericIdentifierRuleID, file, fset.Position(name.Pos()).Line,
					fmt.Sprintf("identifier %q is too generic to explain its role", name.Name), core.ConfidenceHigh))
			}
		}
	}
	return findings
}

func goDefensiveFindings(env support.Context, file string, fset *token.FileSet, body *ast.BlockStmt) []core.Finding {
	findings := make([]core.Finding, 0)
	ast.Inspect(body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.TypeAssertExpr:
			if !goTypeAssertionHasCommaOK(body, node) {
				pos := fset.Position(node.Pos())
				findings = append(findings, precisionWarnFinding(env, defensiveUncheckedTypeAssertionRuleID, file, pos.Line,
					"type assertion is not checked with the comma-ok form", core.ConfidenceHigh))
			}
		case *ast.CallExpr:
			if target := unsafeGoNumericConversionTarget(node); target != "" {
				pos := fset.Position(node.Pos())
				findings = append(findings, precisionWarnFinding(env, defensiveUnsafeNumericConversionRuleID, file, pos.Line,
					fmt.Sprintf("numeric conversion to %s can truncate or wrap; validate bounds before converting", target), core.ConfidenceHigh))
			}
		}
		return true
	})
	return findings
}

func goTypeAssertionHasCommaOK(body *ast.BlockStmt, assertion *ast.TypeAssertExpr) bool {
	checked := false
	ast.Inspect(body, func(n ast.Node) bool {
		if checked {
			return false
		}
		assign, ok := n.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 2 || len(assign.Rhs) != 1 {
			return true
		}
		if assign.Rhs[0] == assertion {
			checked = true
			return false
		}
		return true
	})
	return checked
}

func unsafeGoNumericConversionTarget(call *ast.CallExpr) string {
	if len(call.Args) != 1 {
		return ""
	}
	target, ok := call.Fun.(*ast.Ident)
	if !ok {
		return ""
	}
	switch target.Name {
	case "int8", "int16", "int32", "uint", "uint8", "uint16", "uint32":
	default:
		return ""
	}
	switch call.Args[0].(type) {
	case *ast.BasicLit:
		return ""
	default:
		return target.Name
	}
}

func parsedPrecisionFindings(env support.Context, file string, parsed *support.ParsedFile) []core.Finding {
	functions := parsed.AllFunctions()
	findings := make([]core.Finding, 0, len(functions))
	for _, fn := range functions {
		findings = append(findings, precisionFunctionFindings(env, file, parsedPrecisionFunction(fn))...)
	}
	findings = append(findings, parsedDefensiveFindings(env, file, parsed)...)
	return findings
}

func parsedPrecisionFunction(fn *support.ParsedFunction) precisionFunction {
	body := maskedFunctionBody(fn)
	return precisionFunction{
		Name:        fn.Name,
		StartLine:   fn.StartLine,
		EndLine:     fn.EndLine,
		Signature:   fn.Signature,
		Params:      fn.Params,
		Assignments: fn.Assignments,
		Calls:       fn.Calls,
		Statements:  fn.Statements,
		Body:        body,
		Returns:     strings.Contains(body, "return "),
	}
}

func precisionFunctionFindings(env support.Context, file string, fn precisionFunction) []core.Finding {
	if isQualityFixturePath(file) {
		return nil
	}
	findings := make([]core.Finding, 0)
	if isGenericIdentifier(fn.Name) {
		findings = append(findings, precisionWarnFinding(env, namingGenericIdentifierRuleID, file, fn.StartLine,
			fmt.Sprintf("function name %q is too generic to communicate intent", fn.Name), core.ConfidenceHigh))
	}
	for _, param := range fn.Params {
		if isGenericIdentifier(param.Name) {
			findings = append(findings, precisionWarnFinding(env, namingGenericIdentifierRuleID, file, fn.StartLine,
				fmt.Sprintf("parameter %q is too generic to communicate intent", param.Name), core.ConfidenceHigh))
		}
	}
	for _, assignment := range fn.Assignments {
		if isGenericIdentifier(assignment.Name) {
			findings = append(findings, precisionWarnFinding(env, namingGenericIdentifierRuleID, file, assignment.Line,
				fmt.Sprintf("identifier %q is too generic to explain its role", assignment.Name), core.ConfidenceHigh))
		}
	}
	if mixedAbstractionLevel(fn) {
		findings = append(findings, precisionWarnFinding(env, functionMixedAbstractionLevelRuleID, file, fn.StartLine,
			fmt.Sprintf("function %s mixes orchestration calls with low-level infrastructure operations", fn.Name), core.ConfidenceMedium))
	}
	if commandQueryMix(fn) {
		findings = append(findings, precisionWarnFinding(env, functionCommandQueryMixRuleID, file, fn.StartLine,
			fmt.Sprintf("function %s returns a value while also invoking mutating side-effect operations", fn.Name), core.ConfidenceMedium))
	}
	findings = append(findings, errorHandlingFindings(env, file, fn)...)
	return findings
}

func isGenericIdentifier(name string) bool {
	name = strings.Trim(name, "_$")
	if name == "" {
		return false
	}
	_, ok := genericIdentifierNames[strings.ToLower(name)]
	return ok
}

func mixedAbstractionLevel(fn precisionFunction) bool {
	if fn.EndLine-fn.StartLine < 5 || !lowLevelOperationPattern.MatchString(fn.Body) {
		return false
	}
	for _, call := range fn.Calls {
		if isDomainLevelCall(call.Callee) {
			return true
		}
	}
	return false
}

func isDomainLevelCall(callee string) bool {
	callee = strings.TrimSpace(callee)
	if callee == "" {
		return false
	}
	lowered := strings.ToLower(callee)
	for _, prefix := range []string{"fmt.", "log.", "logger.", "console.", "json.", "json.", "http.", "sql.", "strings.", "strconv.", "errors.", "os.", "fs.", "math.", "time."} {
		if strings.HasPrefix(lowered, prefix) {
			return false
		}
	}
	return strings.Contains(callee, ".") || queryFunctionPrefixPattern.MatchString(lowered) || len(callee) > 3
}

func commandQueryMix(fn precisionFunction) bool {
	if !fn.Returns {
		return false
	}
	name := strings.ToLower(fn.Name)
	if !queryFunctionPrefixPattern.MatchString(name) && !strings.Contains(fn.Body, "return ") {
		return false
	}
	for _, call := range fn.Calls {
		if mutatingCallPattern.MatchString(call.Callee) {
			return true
		}
	}
	return false
}

func errorHandlingFindings(env support.Context, file string, fn precisionFunction) []core.Finding {
	findings := make([]core.Finding, 0)
	statements := fn.Statements
	for idx, statement := range statements {
		line := strings.TrimSpace(statement.Text)
		lowered := strings.ToLower(line)
		if strings.Contains(lowered, "err") || strings.Contains(lowered, "except") || strings.Contains(lowered, "catch") {
			if logsError(line) && nearbyIgnoredError(statements, idx) {
				findings = append(findings, precisionWarnFinding(env, errorLoggedAndIgnoredRuleID, file, statement.Line,
					"error is logged and then ignored or converted to success", core.ConfidenceHigh))
			}
			if returnsBareError(line) || throwsBareError(line) {
				findings = append(findings, precisionWarnFinding(env, errorContextLostRuleID, file, statement.Line,
					"error is returned without contextual wrapping", core.ConfidenceMedium))
			}
		}
	}
	return findings
}

func logsError(line string) bool {
	lowered := strings.ToLower(line)
	return strings.Contains(lowered, "log.") || strings.Contains(lowered, "logger.") || strings.Contains(lowered, "console.error")
}

func nearbyIgnoredError(statements []support.ParsedStatement, idx int) bool {
	for lookahead := idx; lookahead < len(statements) && lookahead <= idx+4; lookahead++ {
		line := strings.TrimSpace(statements[lookahead].Text)
		lowered := strings.ToLower(line)
		if strings.Contains(lowered, "return nil") || strings.Contains(lowered, "return none") ||
			strings.Contains(lowered, "return undefined") || lowered == "return;" ||
			lowered == "pass" || strings.Contains(lowered, "// ignore") {
			return true
		}
		if strings.HasPrefix(lowered, "return ") && !strings.Contains(lowered, "err") && !strings.Contains(lowered, "error") {
			return true
		}
	}
	return false
}

func returnsBareError(line string) bool {
	trimmed := strings.TrimSpace(line)
	return trimmed == "return err" || trimmed == "return err;" || trimmed == "return error" || trimmed == "return error;"
}

func throwsBareError(line string) bool {
	trimmed := strings.TrimSpace(line)
	return trimmed == "throw err;" || trimmed == "throw err" || trimmed == "throw error;" || trimmed == "throw error" ||
		trimmed == "raise err" || trimmed == "raise error"
}

func parsedDefensiveFindings(env support.Context, file string, parsed *support.ParsedFile) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, statement := range parsed.Module.Statements {
		findings = append(findings, defensiveStatementFindings(env, file, statement)...)
	}
	for _, fn := range parsed.AllFunctions() {
		for _, statement := range fn.Statements {
			findings = append(findings, defensiveStatementFindings(env, file, statement)...)
		}
	}
	return findings
}

func defensiveStatementFindings(env support.Context, file string, statement support.ParsedStatement) []core.Finding {
	text := statement.Text
	findings := make([]core.Finding, 0, 2)
	if strings.Contains(text, " as unknown as ") || strings.Contains(text, " as any as ") || strings.Contains(text, "typing.cast(") {
		findings = append(findings, precisionWarnFinding(env, defensiveUncheckedTypeAssertionRuleID, file, statement.Line,
			"type assertion bypasses runtime validation", core.ConfidenceHigh))
	}
	if unsafeScriptNumericConversion(text) {
		findings = append(findings, precisionWarnFinding(env, defensiveUnsafeNumericConversionRuleID, file, statement.Line,
			"numeric conversion can truncate, wrap, or lose precision; validate bounds before converting", core.ConfidenceMedium))
	}
	return findings
}

func unsafeScriptNumericConversion(text string) bool {
	lowered := strings.ToLower(text)
	return strings.Contains(lowered, "static_cast<int8_t>") ||
		strings.Contains(lowered, "static_cast<int16_t>") ||
		strings.Contains(lowered, "static_cast<int32_t>") ||
		strings.Contains(lowered, "static_cast<uint8_t>") ||
		strings.Contains(lowered, "static_cast<uint16_t>") ||
		strings.Contains(lowered, "static_cast<uint32_t>") ||
		strings.Contains(lowered, "number(") && strings.Contains(lowered, "bigint")
}

func maintainabilityDeltaFindings(env support.Context, target core.TargetConfig) []core.Finding {
	if env.Mode != core.ScanModeDiff || env.ReadBaseFile == nil {
		return nil
	}
	changed := changedFilesForTarget(env, target)
	if len(changed) == 0 {
		return nil
	}
	findings := make([]core.Finding, 0)
	for _, rel := range changed {
		if !qualityPrecisionSupportsFile(target.Language, rel) {
			continue
		}
		current, ok := readCurrentTargetFile(env, target, rel)
		if !ok {
			continue
		}
		base, err := env.ReadBaseFile(target, rel)
		if err != nil {
			continue
		}
		line := deltaFindingLine(env, rel)
		findings = append(findings, publicSurfaceGrowthFinding(env, target, rel, base, current, line)...)
		findings = append(findings, dependencyGrowthFinding(env, target, rel, base, current, line)...)
	}
	return findings
}

func changedFilesForTarget(env support.Context, target core.TargetConfig) []string {
	seen := map[string]struct{}{}
	if env.ListChangedFiles != nil {
		if changed, err := env.ListChangedFiles(target); err == nil {
			for _, file := range changed {
				if file.Status == core.ChangedFileDeleted {
					continue
				}
				seen[filepath.ToSlash(file.Path)] = struct{}{}
			}
		}
	}
	if len(seen) == 0 && env.DiffScope != nil {
		for rel := range env.DiffScope() {
			seen[filepath.ToSlash(rel)] = struct{}{}
		}
	}
	paths := make([]string, 0, len(seen))
	for rel := range seen {
		paths = append(paths, rel)
	}
	sort.Strings(paths)
	return paths
}

func readCurrentTargetFile(env support.Context, target core.TargetConfig, rel string) ([]byte, bool) {
	if env.ReadTargetFile != nil {
		if data, err := env.ReadTargetFile(target, rel); err == nil {
			return data, true
		}
	}
	data, err := os.ReadFile(filepath.Join(target.Path, filepath.FromSlash(rel))) //nolint:gosec // rel comes from the scan's own changed-file list
	return data, err == nil
}

func deltaFindingLine(env support.Context, rel string) int {
	if env.DiffScope == nil {
		return 1
	}
	scope := env.DiffScope()[filepath.ToSlash(rel)]
	if scope.AllChanged {
		return 1
	}
	for _, r := range scope.Ranges {
		if r[0] > 0 {
			return r[0]
		}
	}
	return 1
}

func publicSurfaceGrowthFinding(env support.Context, target core.TargetConfig, rel string, base []byte, current []byte, line int) []core.Finding {
	baseCount := publicSurfaceCount(target.Language, rel, string(base))
	currentCount := publicSurfaceCount(target.Language, rel, string(current))
	if currentCount <= baseCount {
		return nil
	}
	return []core.Finding{precisionWarnFinding(env, maintainabilityPublicSurfaceGrowthID, rel, line,
		fmt.Sprintf("public surface grew from %d to %d symbols in this file; keep new exported API intentional", baseCount, currentCount),
		core.ConfidenceHigh)}
}

func dependencyGrowthFinding(env support.Context, target core.TargetConfig, rel string, base []byte, current []byte, line int) []core.Finding {
	baseCount := len(dependencySet(target.Language, rel, string(base)))
	currentCount := len(dependencySet(target.Language, rel, string(current)))
	if currentCount <= baseCount {
		return nil
	}
	return []core.Finding{precisionWarnFinding(env, maintainabilityDependencyGrowthID, rel, line,
		fmt.Sprintf("direct dependencies grew from %d to %d in this file; verify the added dependency surface is necessary", baseCount, currentCount),
		core.ConfidenceHigh)}
}

func qualityPrecisionSupportsFile(language string, rel string) bool {
	switch support.NormalizedLanguage(language) {
	case "", "go":
		return strings.HasSuffix(rel, ".go")
	case "python":
		return strings.HasSuffix(rel, ".py")
	case "typescript", "javascript":
		return isTypeScriptLikeFile(rel)
	case "c++", "cpp":
		return strings.HasSuffix(rel, ".cpp") || strings.HasSuffix(rel, ".cc") || strings.HasSuffix(rel, ".cxx") ||
			strings.HasSuffix(rel, ".hpp") || strings.HasSuffix(rel, ".hh") || strings.HasSuffix(rel, ".h")
	default:
		return false
	}
}

func publicSurfaceCount(language string, rel string, source string) int {
	switch support.NormalizedLanguage(language) {
	case "", "go":
		return len(goPublicDeclPattern.FindAllStringSubmatch(source, -1))
	case "python":
		count := 0
		for _, match := range pythonPublicDeclPattern.FindAllStringSubmatch(source, -1) {
			name := firstNonEmptyString(match[1], match[2])
			if name != "" && !strings.HasPrefix(name, "_") {
				count++
			}
		}
		return count
	case "typescript", "javascript":
		return len(tsPublicDeclPattern.FindAllStringSubmatch(source, -1))
	case "c++", "cpp":
		if !strings.HasSuffix(rel, ".h") && !strings.HasSuffix(rel, ".hh") && !strings.HasSuffix(rel, ".hpp") {
			return 0
		}
		return len(cppPublicDeclPattern.FindAllStringSubmatch(source, -1))
	default:
		return 0
	}
}

func dependencySet(language string, rel string, source string) map[string]struct{} {
	deps := map[string]struct{}{}
	switch support.NormalizedLanguage(language) {
	case "", "go":
		if fset, parsed, err := support.ParseGoSource(support.Context{}, rel, []byte(source)); err == nil {
			_ = fset
			for _, imp := range parsed.Imports {
				deps[strings.Trim(imp.Path.Value, `"`)] = struct{}{}
			}
		}
	case "python":
		for _, imp := range support.ParsePython(source).Imports {
			deps[firstNonEmptyString(imp.Module, imp.Name, imp.Alias)] = struct{}{}
		}
	case "typescript", "javascript":
		for _, imp := range support.ParseCLike(source, support.CLikeTypeScript).Imports {
			deps[imp.Module] = struct{}{}
		}
	case "c++", "cpp":
		for _, match := range cppIncludePattern.FindAllStringSubmatch(source, -1) {
			deps[match[1]] = struct{}{}
		}
	}
	return deps
}

func isQualityFixturePath(path string) bool {
	normalized := strings.ToLower(filepath.ToSlash(path))
	if strings.Contains(normalized, "/testdata/") || strings.Contains(normalized, "/fixtures/") || strings.Contains(normalized, "/__fixtures__/") {
		return true
	}
	return strings.HasSuffix(normalized, "_test.go") || strings.HasSuffix(normalized, "_test.py") ||
		strings.HasSuffix(normalized, ".test.ts") || strings.HasSuffix(normalized, ".spec.ts") ||
		strings.HasSuffix(normalized, ".test.js") || strings.HasSuffix(normalized, ".spec.js")
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
