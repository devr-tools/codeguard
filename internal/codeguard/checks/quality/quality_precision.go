package quality

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"path/filepath"
	"regexp"
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
	qualityDuplicatedKnowledgeRuleID       = "quality.duplicated-knowledge"
	qualityAmbiguousNameRuleID             = "quality.ambiguous-name"
	qualityBooleanArgumentRuleID           = "quality.boolean-argument"
	qualityMixedAbstractionLevelsRuleID    = "quality.mixed-abstraction-levels"
	qualityPrimitiveObsessionRuleID        = "quality.primitive-obsession"
	qualityHiddenSideEffectRuleID          = "quality.hidden-side-effect"
	qualityMutableGlobalStateRuleID        = "quality.mutable-global-state"
	qualityRedundantCommentRuleID          = "quality.redundant-comment"
)

var (
	genericIdentifierNames = map[string]struct{}{
		"foo": {}, "bar": {}, "baz": {}, "qux": {},
		"tmp": {}, "temp": {}, "thing": {}, "stuff": {}, "obj": {}, "misc": {},
	}
	ambiguousIdentifierNames = map[string]struct{}{
		"manager": {}, "helper": {}, "helpers": {}, "process": {}, "processor": {},
		"thing": {}, "object": {}, "util": {}, "utils": {},
		"misc": {}, "stuff": {},
	}
	queryFunctionPrefixPattern = regexp.MustCompile(`^(get|find|list|load|read|lookup|fetch|is|has|can|should|compute|calculate|build|format|parse)`)
	mutatingCallPattern        = regexp.MustCompile(`(?i)(^|[.>:\-_])(add|allocate|append|assign|clear|create|delete|emit|insert|mutate|persist|pop|publish|push|push_back|remove|reverse|save|send|set|sort|splice|store|update|upsert|write)([A-Z_:\-.]|$)`)
	lowLevelOperationPattern   = regexp.MustCompile(`(?i)(\bsql\.|\.query\(|\.exec\(|\bhttp\.|\bfetch\(|\baxios\.|\brequests\.|\bjson\.|\bJSON\.|\bos\.Getenv\b|\bprocess\.env\b|\bfs\.|#include\b)`)
	primitiveTypePattern       = regexp.MustCompile(`(?i)\b(string|str|int|int64|float|float64|double|decimal|number|boolean|bool|char|long|short)\b`)
	domainPrimitiveNamePattern = regexp.MustCompile(`(?i)(id|status|state|type|kind|currency|amount|price|email|phone|country|role|permission|tenant|account|customer|order)`)
	mutableGlobalLinePattern   = regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:let|var)\s+[A-Za-z_$][\w$]*\s*=|^\s*[A-Za-z_]\w*\s*=`)
	redundantCommentPattern    = regexp.MustCompile(`(?i)^\s*(//|#)\s*(get|set|create|delete|update|save|return|initialize|validate|parse|build|handle)\b`)
	goPublicDeclPattern        = regexp.MustCompile(`(?m)^(?:func|type|var|const)\s+(?:\([^)]*\)\s*)?([A-Z][A-Za-z0-9_]*)\b`)
	pythonPublicDeclPattern    = regexp.MustCompile(`(?m)^class\s+([A-Za-z]\w*)\b|^def\s+([A-Za-z]\w*)\s*\(`)
	tsPublicDeclPattern        = regexp.MustCompile(`(?m)^export\s+(?:declare\s+)?(?:async\s+)?(?:class|interface|type|enum|function|const|let|var)\s+([A-Za-z_$][\w$]*)\b`)
	cppPublicDeclPattern       = regexp.MustCompile(`(?m)^\s*(?:class|struct)\s+([A-Z]\w*)\b|^\s*(?:[A-Za-z_][\w:<>,\s*&~]*\s+)+([A-Z]\w*)\s*\([^;{}]*\)\s*;`)
	cppIncludePattern          = regexp.MustCompile(`(?m)^\s*#include\s+[<"]([^>"]+)[>"]`)
)

type precisionFunction struct {
	Name                         string
	Receiver                     string
	StartLine                    int
	EndLine                      int
	Signature                    string
	Params                       []support.ParsedParam
	Assignments                  []support.ParsedAssignment
	Calls                        []support.ParsedCall
	Statements                   []support.ParsedStatement
	Nested                       []precisionLineRange
	Body                         string
	Returns                      bool
	ImplementsInterfaceSignature bool
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
	interfaceMethods := goInterfaceMethodSignatures(parsed)
	ast.Inspect(parsed, func(n ast.Node) bool {
		if node, ok := n.(*ast.FuncDecl); ok {
			fn := goPrecisionFunction(fset, node, data)
			fn.ImplementsInterfaceSignature = interfaceMethods[goInterfaceMethodKey(fn.Name, fn.Params, fn.Signature)]
			findings = append(findings, precisionFunctionFindings(env, file, fn)...)
			if node.Body != nil {
				findings = append(findings, goDefensiveFindings(env, file, fset, node.Body)...)
			}
		}
		return true
	})
	for _, decl := range parsed.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		findings = append(findings, goGenericDeclFindings(env, file, fset, gen)...)
		findings = append(findings, goMutableGlobalFindings(env, file, fset, parsed, gen)...)
		findings = append(findings, goDuplicatedKnowledgeFindings(env, file, fset, gen)...)
	}
	findings = append(findings, redundantCommentFindings(env, file, string(data))...)
	findings = append(findings, sourceDuplicatedKnowledgeFindings(env, file, string(data))...)
	findings = append(findings, sourceNamingFindings(env, file, string(data))...)
	findings = append(findings, sourceDefensiveInvariantFindings(env, file, string(data))...)
	return findings
}

func goPrecisionFunction(fset *token.FileSet, fn *ast.FuncDecl, data []byte) precisionFunction {
	out := precisionFunction{
		Name:      fn.Name.Name,
		Receiver:  goReceiverType(fn),
		StartLine: fset.Position(fn.Pos()).Line,
		EndLine:   fset.Position(fn.End()).Line,
		Signature: goResultSignature(fn),
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
		case *ast.FuncLit:
			out.Nested = append(out.Nested, precisionLineRange{
				Start: fset.Position(node.Pos()).Line,
				End:   fset.Position(node.End()).Line,
			})
			return false
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

func goResultSignature(fn *ast.FuncDecl) string {
	if fn.Type == nil || fn.Type.Results == nil || len(fn.Type.Results.List) == 0 {
		return ""
	}
	results := make([]string, 0, len(fn.Type.Results.List))
	for _, field := range fn.Type.Results.List {
		text := goExprText(field.Type)
		if len(field.Names) == 0 {
			results = append(results, text)
			continue
		}
		for range field.Names {
			results = append(results, text)
		}
	}
	if len(results) == 1 {
		return results[0]
	}
	return "(" + strings.Join(results, ", ") + ")"
}

func goParsedParams(fn *ast.FuncDecl) []support.ParsedParam {
	if fn.Type == nil || fn.Type.Params == nil {
		return nil
	}
	params := make([]support.ParsedParam, 0)
	for _, field := range fn.Type.Params.List {
		typ := ""
		typ = goExprText(field.Type)
		for _, name := range field.Names {
			params = append(params, support.ParsedParam{Name: name.Name, Type: typ})
		}
		if len(field.Names) == 0 {
			params = append(params, support.ParsedParam{Type: typ})
		}
	}
	return params
}

func goReceiverType(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}
	return strings.TrimPrefix(goExprText(fn.Recv.List[0].Type), "*")
}

func goInterfaceMethodSignatures(parsed *ast.File) map[string]bool {
	out := map[string]bool{}
	for _, decl := range parsed.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			iface, ok := typeSpec.Type.(*ast.InterfaceType)
			if !ok || iface.Methods == nil {
				continue
			}
			for _, method := range iface.Methods.List {
				methodType, ok := method.Type.(*ast.FuncType)
				if !ok {
					continue
				}
				methodDecl := &ast.FuncDecl{Type: methodType}
				signature := goResultSignature(methodDecl)
				params := goParsedParams(methodDecl)
				for _, name := range method.Names {
					out[goInterfaceMethodKey(name.Name, params, signature)] = true
				}
			}
		}
	}
	return out
}

func goInterfaceMethodKey(name string, params []support.ParsedParam, signature string) string {
	parts := make([]string, 0, len(params)+2)
	parts = append(parts, name)
	for _, param := range params {
		parts = append(parts, strings.ReplaceAll(param.Type, " ", ""))
	}
	parts = append(parts, strings.ReplaceAll(signature, " ", ""))
	return strings.Join(parts, "\x00")
}

func goExprText(expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	var buf bytes.Buffer
	_ = printer.Fprint(&buf, token.NewFileSet(), expr)
	return buf.String()
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

type globalDeclarationClassification struct {
	immutableByConvention   bool
	technicallyReassignable bool
}

func goMutableGlobalFindings(env support.Context, file string, fset *token.FileSet, parsed *ast.File, decl *ast.GenDecl) []core.Finding {
	if decl.Tok != token.VAR || isQualityFixturePath(file) {
		return nil
	}
	findings := make([]core.Finding, 0)
	for _, spec := range decl.Specs {
		value, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for idx, name := range value.Names {
			if strings.HasPrefix(strings.ToLower(name.Name), "err") {
				continue
			}
			classification := classifyGoGlobalDeclaration(value, idx)
			if classification.immutableByConvention && classification.technicallyReassignable && !goGlobalIsReassigned(parsed, name.Name) {
				continue
			}
			message := fmt.Sprintf("mutable package-level variable %q makes behavior harder to isolate and test", name.Name)
			if classification.immutableByConvention {
				message = fmt.Sprintf("package-level variable %q is reassigned after immutable construction", name.Name)
			}
			findings = append(findings, precisionWarnFinding(env, qualityMutableGlobalStateRuleID, file, fset.Position(name.Pos()).Line,
				message, core.ConfidenceHigh))
		}
	}
	return findings
}

func classifyGoGlobalDeclaration(spec *ast.ValueSpec, index int) globalDeclarationClassification {
	classification := globalDeclarationClassification{technicallyReassignable: true}
	if len(spec.Values) == 0 {
		return classification
	}
	exprIndex := index
	if exprIndex >= len(spec.Values) {
		exprIndex = len(spec.Values) - 1
	}
	call, ok := spec.Values[exprIndex].(*ast.CallExpr)
	if !ok {
		return classification
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "MustCompile" {
		return classification
	}
	pkg, ok := selector.X.(*ast.Ident)
	classification.immutableByConvention = ok && pkg.Name == "regexp"
	return classification
}

func goGlobalIsReassigned(parsed *ast.File, name string) bool {
	reassigned := false
	ast.Inspect(parsed, func(node ast.Node) bool {
		assignment, ok := node.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for _, lhs := range assignment.Lhs {
			if ident, ok := lhs.(*ast.Ident); ok && ident.Name == name {
				reassigned = true
				return false
			}
		}
		return true
	})
	return reassigned
}

func goDuplicatedKnowledgeFindings(env support.Context, file string, fset *token.FileSet, decl *ast.GenDecl) []core.Finding {
	if decl.Tok != token.CONST || isQualityFixturePath(file) {
		return nil
	}
	seen := map[string]token.Pos{}
	for _, spec := range decl.Specs {
		value, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for _, expr := range value.Values {
			lit, ok := expr.(*ast.BasicLit)
			if !ok || !domainKnowledgeLiteral(lit.Value) {
				continue
			}
			if first, exists := seen[lit.Value]; exists {
				return []core.Finding{precisionWarnFinding(env, qualityDuplicatedKnowledgeRuleID, file, fset.Position(expr.Pos()).Line,
					fmt.Sprintf("business literal %s is duplicated near line %d; centralize shared domain knowledge", lit.Value, fset.Position(first).Line), core.ConfidenceLow)}
			}
			seen[lit.Value] = expr.Pos()
		}
	}
	return nil
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
	findings = append(findings, parsedMutableGlobalFindings(env, file, parsed)...)
	findings = append(findings, parsedDuplicatedKnowledgeFindings(env, file, parsed)...)
	findings = append(findings, sourceMutableGlobalFindings(env, file, parsed.Source)...)
	findings = append(findings, sourceDuplicatedKnowledgeFindings(env, file, parsed.Source)...)
	findings = append(findings, redundantCommentFindings(env, file, parsed.Source)...)
	findings = append(findings, sourceNamingFindings(env, file, parsed.Source)...)
	findings = append(findings, sourceDefensiveInvariantFindings(env, file, parsed.Source)...)
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
		Nested:      nestedPrecisionLineRanges(fn),
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
	if isAmbiguousIdentifier(fn.Name) && !isUIConventionalAmbiguousName(file, fn, fn.Name, "", fn.StartLine) && !isLocallyClearAmbiguousName(fn, fn.Name) {
		findings = append(findings, precisionWarnFinding(env, qualityAmbiguousNameRuleID, file, fn.StartLine,
			fmt.Sprintf("function name %q is ambiguous without domain context", fn.Name), core.ConfidenceHigh))
	}
	for _, param := range fn.Params {
		if isGenericIdentifier(param.Name) && !isUIHelperOrMappingContext(file, fn) && !isSeedOrScriptSourcePath(file) {
			findings = append(findings, precisionWarnFinding(env, namingGenericIdentifierRuleID, file, fn.StartLine,
				fmt.Sprintf("parameter %q is too generic to communicate intent", param.Name), core.ConfidenceHigh))
		}
		if isAmbiguousIdentifier(param.Name) && !isUIConventionalAmbiguousName(file, fn, param.Name, param.Type, fn.StartLine) && !isLocallyClearAmbiguousName(fn, param.Name) {
			findings = append(findings, precisionWarnFinding(env, qualityAmbiguousNameRuleID, file, fn.StartLine,
				fmt.Sprintf("parameter %q is ambiguous without domain context", param.Name), core.ConfidenceHigh))
		}
		if isBooleanParameter(param) && !isAllowedBooleanArgumentFunction(fn.Name) && !isPredicateName(param.Name) && !isConventionalNonPredicateName(param.Name) &&
			!isLocalBooleanParserFlag(fn, param.Name) &&
			!isReactComponentOrHookBoundary(file, fn) && !isUIHelperOrMappingContext(file, fn) && !isSeedOrScriptSourcePath(file) {
			findings = append(findings, precisionWarnFinding(env, qualityBooleanArgumentRuleID, file, fn.StartLine,
				fmt.Sprintf("boolean parameter %q hides behavior behind a flag", param.Name), core.ConfidenceHigh))
		}
	}
	for _, assignment := range fn.Assignments {
		if isGenericIdentifier(assignment.Name) && !isUIHelperOrMappingContext(file, fn) && !isSeedOrScriptSourcePath(file) {
			findings = append(findings, precisionWarnFinding(env, namingGenericIdentifierRuleID, file, assignment.Line,
				fmt.Sprintf("identifier %q is too generic to explain its role", assignment.Name), core.ConfidenceHigh))
		}
		if isAmbiguousIdentifier(assignment.Name) && !isUIConventionalAmbiguousName(file, fn, assignment.Name, "", assignment.Line) && !isLocallyClearAmbiguousName(fn, assignment.Name) {
			findings = append(findings, precisionWarnFinding(env, qualityAmbiguousNameRuleID, file, assignment.Line,
				fmt.Sprintf("identifier %q is ambiguous without domain context", assignment.Name), core.ConfidenceHigh))
		}
	}
	if mixedAbstractionLevel(fn) &&
		!isQualityFixturePath(file) &&
		!isPostgresRepositoryPath(file) &&
		!isAdapterOrOrchestrationFunction(file, fn) &&
		!isFrameworkOrchestrationBoundary(file, fn) &&
		!isScriptEntrypoint(file, fn.Name) &&
		!isSeedOrScriptSourcePath(file) &&
		!isSecurityOrConfigUtilityFunction(file, fn) &&
		!isDomainSideEffectBoundaryName(fn.Name) &&
		!isFactoryHelperName(fn.Name) &&
		!isPureComputationHelperName(fn.Name) &&
		!isReactComponentOrHookBoundary(file, fn) &&
		!isUIHelperOrMappingContext(file, fn) {
		findings = append(findings, precisionWarnFinding(env, functionMixedAbstractionLevelRuleID, file, fn.StartLine,
			fmt.Sprintf("function %s mixes orchestration calls with low-level infrastructure operations", fn.Name), core.ConfidenceMedium))
	}
	if commandQueryMix(file, fn) {
		findings = append(findings, precisionWarnFinding(env, functionCommandQueryMixRuleID, file, fn.StartLine,
			fmt.Sprintf("function %s returns a value while also invoking mutating side-effect operations", fn.Name), core.ConfidenceMedium))
	}
	findings = append(findings, additionalPrecisionFunctionFindings(env, file, fn)...)
	if !isUIHelperOrMappingContext(file, fn) && !isSeedOrScriptSourcePath(file) && !isFrontendLibraryPath(file) &&
		!isDomainSideEffectBoundaryName(fn.Name) && !isValidationOrExtractionHelperName(fn.Name) && primitiveObsession(fn) {
		if !fn.ImplementsInterfaceSignature {
			findings = append(findings, precisionWarnFinding(env, qualityPrimitiveObsessionRuleID, file, fn.StartLine,
				fmt.Sprintf("function %s passes several domain concepts as raw primitives", fn.Name), core.ConfidenceMedium))
		}
	}
	if hiddenSideEffect(file, fn) {
		findings = append(findings, precisionWarnFinding(env, qualityHiddenSideEffectRuleID, file, fn.StartLine,
			fmt.Sprintf("function %s name implies a query/build operation but it performs side effects", fn.Name), core.ConfidenceMedium))
	}
	findings = append(findings, errorHandlingFindings(env, file, fn)...)
	findings = append(findings, errorContractFindings(env, file, fn)...)
	findings = append(findings, defensiveBoundaryFindings(env, file, fn)...)
	return findings
}

func isLocalBooleanParserFlag(fn precisionFunction, name string) bool {
	loweredName := strings.ToLower(strings.Trim(name, "_$"))
	if loweredName != "lower" && loweredName != "trim" && loweredName != "strict" {
		return false
	}
	loweredFunction := strings.ToLower(strings.Trim(fn.Name, "_$"))
	return containsAny(loweredFunction, []string{"parse", "split", "normalize", "format", "read"})
}

func isGenericIdentifier(name string) bool {
	name = strings.Trim(name, "_$")
	if name == "" {
		return false
	}
	_, ok := genericIdentifierNames[strings.ToLower(name)]
	return ok
}

func isAmbiguousIdentifier(name string) bool {
	name = strings.Trim(name, "_$")
	if name == "" {
		return false
	}
	_, ok := ambiguousIdentifierNames[strings.ToLower(name)]
	return ok
}

func isLocallyClearAmbiguousName(fn precisionFunction, name string) bool {
	normalized := strings.ToLower(strings.Trim(name, "_$"))
	if normalized != "value" && normalized != "values" {
		return false
	}
	loweredName := strings.ToLower(fn.Name)
	return containsAny(loweredName, []string{"parse", "normalize", "format", "render", "map", "transform", "compare", "equal", "record", "field", "option"})
}

func isBooleanParameter(param support.ParsedParam) bool {
	typ := strings.TrimSpace(param.Type)
	if strings.Contains(typ, "{") || strings.Contains(typ, "}") {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(param.Type), "bool") ||
		strings.EqualFold(strings.TrimSpace(param.Type), "boolean") ||
		strings.Contains(strings.ToLower(param.Type), " bool") ||
		strings.Contains(strings.ToLower(param.Type), ": boolean")
}

func isAllowedBooleanArgumentFunction(name string) bool {
	lowered := strings.ToLower(name)
	return strings.HasPrefix(lowered, "set") || strings.HasPrefix(lowered, "with") ||
		strings.HasPrefix(lowered, "enable") || strings.HasPrefix(lowered, "disable") ||
		strings.Contains(lowered, "option")
}

func primitiveObsession(fn precisionFunction) bool {
	count := 0
	for _, param := range fn.Params {
		if primitiveTypePattern.MatchString(param.Type) && domainPrimitiveNamePattern.MatchString(param.Name) {
			count++
		}
	}
	return count >= 3
}

func hiddenSideEffect(file string, fn precisionFunction) bool {
	if isFrameworkOrchestrationBoundary(file, fn) || isReactComponentOrNamedHookBoundary(file, fn) || isUIHelperOrMappingContext(file, fn) || isSeedOrScriptSourcePath(file) || isAdapterOrOrchestrationFunction(file, fn) || isSecurityOrConfigUtilityFunction(file, fn) || explicitMutationName(fn.Name) || isUICommandHelperName(file, fn.Name) {
		return false
	}
	if isDomainSideEffectBoundaryName(fn.Name) {
		return false
	}
	if isPureComputationHelperName(fn.Name) && !hasPersistentCollaboratorSideEffect(fn) {
		return false
	}
	if !queryFunctionPrefixPattern.MatchString(strings.ToLower(fn.Name)) {
		return false
	}
	if isAccumulatorBuilderFunctionName(fn.Name) && !hasLikelyExternalMutationCall(fn) {
		return false
	}
	localTargets := localMutationTargets(fn)
	for _, call := range directCalls(fn) {
		if mutatingCallPattern.MatchString(call.Callee) && !isLocalMutationCall(call, localTargets) && !isLocalBuilderMutationCall(fn, call) && !isBuilderAccumulatorMutationCall(fn, call) {
			return true
		}
	}
	return false
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

func commandQueryMix(file string, fn precisionFunction) bool {
	if isQualityFixturePath(file) {
		return false
	}
	if explicitRepositoryCommandResultContract(file, fn) {
		return false
	}
	if isFrameworkOrchestrationBoundary(file, fn) || isReactComponentOrNamedHookBoundary(file, fn) || isUIHelperOrMappingContext(file, fn) || isScriptEntrypoint(file, fn.Name) || isSeedOrScriptSourcePath(file) || isAdapterOrOrchestrationFunction(file, fn) || isPostgresRepositoryPath(file) || isSecurityOrConfigUtilityFunction(file, fn) || explicitMutationName(fn.Name) || isUICommandHelperName(file, fn.Name) || isDomainSideEffectBoundaryName(fn.Name) {
		return false
	}
	if !fn.Returns {
		return false
	}
	if isAccumulatorBuilderFunctionName(fn.Name) && !hasLikelyExternalMutationCall(fn) {
		return false
	}
	if isFactoryHelperName(fn.Name) {
		return false
	}
	if isPureComputationHelperName(fn.Name) && !hasLikelyParameterAssignment(fn) && !hasIOOrPersistenceSideEffect(fn) {
		return false
	}
	if isUIHelperOrMappingContext(file, fn) && !hasLikelyParameterAssignment(fn) && !predicateHasObviousSideEffect(fn) {
		return false
	}
	if isPredicateName(fn.Name) && !hasLikelyParameterAssignment(fn) && !predicateHasObviousSideEffect(fn) {
		return false
	}
	name := strings.ToLower(fn.Name)
	if !queryFunctionPrefixPattern.MatchString(name) && !strings.Contains(fn.Body, "return ") {
		return false
	}
	localTargets := localMutationTargets(fn)
	for _, call := range directCalls(fn) {
		if mutatingCallPattern.MatchString(call.Callee) && !isLocalMutationCall(call, localTargets) && !isLocalBuilderMutationCall(fn, call) && !isBuilderAccumulatorMutationCall(fn, call) {
			return true
		}
	}
	return false
}

func errorHandlingFindings(env support.Context, file string, fn precisionFunction) []core.Finding {
	if isUIHelperOrMappingContext(file, fn) || isSeedOrScriptSourcePath(file) {
		return nil
	}
	findings := make([]core.Finding, 0)
	statements := fn.Statements
	for idx, statement := range statements {
		line := strings.TrimSpace(statement.Text)
		lowered := strings.ToLower(line)
		if strings.Contains(lowered, "err") || strings.Contains(lowered, "except") || strings.Contains(lowered, "catch") {
			if logsError(line) && nearbyIgnoredError(statements, idx) && !nearbyExplicitDegradedFallback(statements, idx) {
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

func nearbyExplicitDegradedFallback(statements []support.ParsedStatement, idx int) bool {
	for lookahead := idx; lookahead < len(statements) && lookahead <= idx+4; lookahead++ {
		line := strings.ToLower(strings.TrimSpace(firstNonEmptyString(statements[lookahead].Raw, statements[lookahead].Text)))
		if containsAny(line, []string{
			"return []", "return {}", "return reports", "return fallback", "return default",
			"return;",
		}) {
			return true
		}
	}
	return false
}

func logsError(line string) bool {
	lowered := strings.ToLower(line)
	return strings.Contains(lowered, "log.") || strings.Contains(lowered, "logger.") ||
		strings.Contains(lowered, "logging.") || strings.Contains(lowered, "console.error")
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
		if !isScriptTestOrHelperFile(file) {
			findings = append(findings, precisionWarnFinding(env, defensiveUncheckedTypeAssertionRuleID, file, statement.Line,
				"type assertion bypasses runtime validation", core.ConfidenceHigh))
		}
	}
	if unsafeScriptNumericConversion(text) {
		findings = append(findings, precisionWarnFinding(env, defensiveUnsafeNumericConversionRuleID, file, statement.Line,
			"numeric conversion can truncate, wrap, or lose precision; validate bounds before converting", core.ConfidenceMedium))
	}
	return findings
}

func parsedMutableGlobalFindings(env support.Context, file string, parsed *support.ParsedFile) []core.Finding {
	if isQualityFixturePath(file) {
		return nil
	}
	normalized := strings.ToLower(strings.ReplaceAll(file, "\\", "/"))
	if strings.HasPrefix(normalized, "bin/") || strings.Contains(normalized, "/integrations/") || strings.Contains(normalized, "integrations/") {
		return nil
	}
	findings := make([]core.Finding, 0)
	for _, statement := range parsed.Module.Statements {
		text := strings.TrimSpace(statement.Text)
		if text == "" || strings.HasPrefix(text, "const ") || strings.HasPrefix(text, "final ") {
			continue
		}
		if isScriptLikeSourcePath(file) && !moduleStatementLooksTopLevel(statement) {
			continue
		}
		if mutableGlobalLinePattern.MatchString(text) {
			findings = append(findings, precisionWarnFinding(env, qualityMutableGlobalStateRuleID, file, statement.Line,
				"mutable module-level state makes behavior harder to isolate and test", core.ConfidenceHigh))
		}
	}
	return findings
}

func redundantCommentFindings(env support.Context, file string, source string) []core.Finding {
	if isQualityFixturePath(file) {
		return nil
	}
	if isSeedOrScriptSourcePath(file) {
		return nil
	}
	lines := strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n")
	for idx := 0; idx+1 < len(lines); idx++ {
		comment := strings.TrimSpace(lines[idx])
		next := strings.TrimSpace(lines[idx+1])
		if !redundantCommentPattern.MatchString(comment) || next == "" {
			continue
		}
		verb := redundantCommentVerb(comment)
		if verb != "" && strings.Contains(strings.ToLower(next), verb) {
			return []core.Finding{precisionWarnFinding(env, qualityRedundantCommentRuleID, file, idx+1,
				"comment restates the next line without adding design intent or constraints", core.ConfidenceLow)}
		}
	}
	return nil
}

func sourceMutableGlobalFindings(env support.Context, file string, source string) []core.Finding {
	if isQualityFixturePath(file) {
		return nil
	}
	if strings.EqualFold(filepath.Ext(file), ".go") {
		return nil
	}
	normalized := strings.ToLower(strings.ReplaceAll(file, "\\", "/"))
	if strings.HasPrefix(normalized, "bin/") || strings.Contains(normalized, "/integrations/") || strings.Contains(normalized, "integrations/") {
		return nil
	}
	for idx, line := range strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n") {
		if isScriptLikeSourcePath(file) && !scriptSourceLineAtModuleScope(source, idx) {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") ||
			strings.HasPrefix(trimmed, "const ") || strings.HasPrefix(trimmed, "final ") {
			continue
		}
		if mutableGlobalLinePattern.MatchString(trimmed) && !strings.Contains(trimmed, " := ") {
			return []core.Finding{precisionWarnFinding(env, qualityMutableGlobalStateRuleID, file, idx+1,
				"mutable module-level state makes behavior harder to isolate and test", core.ConfidenceHigh)}
		}
	}
	return nil
}

func redundantCommentVerb(comment string) string {
	match := redundantCommentPattern.FindStringSubmatch(comment)
	if len(match) < 3 {
		return ""
	}
	return strings.ToLower(match[2])
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
