package design

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

const (
	ruleShallowModule          = "design.shallow-module"
	ruleExcessivePublicSurface = "design.excessive-public-surface"
	rulePassThrough            = "design.pass-through-abstraction" // #nosec G101 -- rule id, not a credential.
	ruleConfigurationLeak      = "design.configuration-leak"
	ruleTemporalCoupling       = "design.temporal-coupling"
	ruleInfrastructureLeak     = "design.infrastructure-type-leak"
	rulePersistenceLeak        = "design.persistence-model-leak"
	ruleDomainLogicInHandler   = "design.domain-logic-in-handler"
)

var (
	tsExportPattern        = regexp.MustCompile(`(?m)^\s*export\s+(?:async\s+)?(?:class|interface|type|enum|function|const|let|var)\s+([A-Za-z_$][\w$]*)\b`)
	pythonPublicPattern    = regexp.MustCompile(`(?m)^\s*(?:class|def)\s+([A-Za-z]\w*)\b`)
	cppPublicPattern       = regexp.MustCompile(`(?m)^\s*(?:class|struct)\s+([A-Z]\w*)\b|^\s*(?:[A-Za-z_][\w:<>,\s*&~]*\s+)+([A-Z]\w*)\s*\([^;{}]*\)\s*;`)
	delegationCallPattern  = regexp.MustCompile(`(?i)\b(delegate|client|inner|wrapped|service|repo|repository|store|gateway|adapter|api|impl)\s*(?:\.|->|::)\s*[A-Za-z_]\w*\s*\(`)
	infraLeakPattern       = regexp.MustCompile(`(?i)\b(sql\.|http\.Request|http\.ResponseWriter|gin\.Context|echo\.Context|fiber\.Ctx|gorm\.DB|redis\.Client|kafka\.|sqs\.|sns\.|boto3|requests\.|express\.Request|Request<|Response<|PDO|mysqli|std::istream|std::ostream)\b`)
	persistenceLeakPattern = regexp.MustCompile(
		`(?i)\b(db|dao|dto|entity|record|row|orm|model)\b|[A-Za-z_]*(DTO|Entity|Model|Record|Row)\b|gorm:|sequelize|typeorm|sqlalchemy|django\.db|ActiveRecord|@Entity|Prisma\.`,
	)
	configLeakPattern     = regexp.MustCompile(`(?i)\b(os\.Getenv|process\.env|getenv\(|System\.getenv|Config\b|Settings\b|Options\b|FeatureFlag|ENV\[)`)
	domainTermPattern     = regexp.MustCompile(`(?i)\b(discount|price|amount|currency|inventory|permission|role|status|quota|eligib|payment|order|invoice|customer|account)\b`)
	mutationPattern       = regexp.MustCompile(`(?i)\b(save|insert|update|delete|charge|refund|publish|emit|commit|execute|query)\b`)
	temporalSetupPattern  = regexp.MustCompile(`(?i)\b(init|initialize|configure|connect|open|set[A-Z_]|begin|prepare)\b`)
	temporalActionPattern = regexp.MustCompile(`(?i)\b(start|run|execute|send|publish|commit|close|flush|use)\b`)
)

type designFunction struct {
	Name       string
	StartLine  int
	Statements []support.ParsedStatement
	Calls      []support.ParsedCall
}

type publicSymbol struct {
	Name string
	Line int
}

func localAbstractionFindings(env support.Context, target core.TargetConfig) []core.Finding {
	return env.ScanTargetFiles(target, "design", func(rel string) bool {
		return localDesignSupportsFile(target.Language, rel)
	}, func(file string, data []byte) []core.Finding {
		return localDesignFileFindings(env, target, file, data)
	})
}

func localDesignSupportsFile(language string, rel string) bool {
	switch support.NormalizedLanguage(language) {
	case "", "go":
		return strings.HasSuffix(rel, ".go")
	case "python", "py":
		return strings.HasSuffix(rel, ".py")
	case "typescript", "javascript", "ts", "tsx", "js", "jsx":
		return isTypeScriptLikeFile(rel)
	case "c++", "cpp", "cxx", "cc":
		return strings.HasSuffix(rel, ".cpp") || strings.HasSuffix(rel, ".cc") || strings.HasSuffix(rel, ".cxx") ||
			strings.HasSuffix(rel, ".hpp") || strings.HasSuffix(rel, ".hh") || strings.HasSuffix(rel, ".h")
	default:
		return false
	}
}

func localDesignFileFindings(env support.Context, target core.TargetConfig, file string, data []byte) []core.Finding {
	source := strings.ReplaceAll(string(data), "\r\n", "\n")
	symbols := publicSymbols(target.Language, file, source)
	functions := designFunctions(env, target.Language, file, data)
	findings := make([]core.Finding, 0, len(functions)+5)
	findings = append(findings, localPublicSurfaceFindings(env, file, symbols, functions, source)...)
	findings = append(findings, leakFindings(env, file, source)...)
	for _, fn := range functions {
		findings = append(findings, functionAbstractionFindings(env, file, fn)...)
	}
	return findings
}

func localPublicSurfaceFindings(env support.Context, file string, symbols []publicSymbol, functions []designFunction, source string) []core.Finding {
	findings := make([]core.Finding, 0, 2)
	maxPublic := max(1, env.Config.Checks.DesignRules.MaxDeclsPerFile)
	if len(symbols) > maxPublic {
		findings = append(findings, designFinding(env, ruleExcessivePublicSurface, file, 1,
			fmt.Sprintf("file exposes %d public symbols; max is %d", len(symbols), maxPublic), core.ConfidenceHigh))
	}
	shallowThreshold := max(2, env.Config.Checks.DesignRules.MaxInterfaceMethods)
	if len(symbols) >= shallowThreshold && averageFunctionStatements(functions) <= 1 && exportedWrapperDensity(source) >= 2 {
		findings = append(findings, designFinding(env, ruleShallowModule, file, 1,
			fmt.Sprintf("module exposes %d public symbols but most behavior is shallow delegation or declarations", len(symbols)), core.ConfidenceLow))
	}
	return findings
}

func leakFindings(env support.Context, file string, source string) []core.Finding {
	lines := strings.Split(source, "\n")
	findings := make([]core.Finding, 0, 3)
	domainPath := isDomainPath(file)
	apiPath := isAPIPath(file)
	handlerPath := isHandlerPath(file)
	for idx, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") {
			continue
		}
		lineNo := idx + 1
		if domainPath && infraLeakPattern.MatchString(trimmed) {
			findings = append(findings, designFinding(env, ruleInfrastructureLeak, file, lineNo,
				"infrastructure/framework type leaks into a domain or public boundary", core.ConfidenceHigh))
		}
		if (apiPath || handlerPath || isPublicDeclaration(trimmed)) && persistenceLeakPattern.MatchString(trimmed) &&
			!allowedGeneratedPersistenceEnumLine(trimmed) && !allowedTypeScriptRecordUtilityLine(trimmed) {
			findings = append(findings, designFinding(env, rulePersistenceLeak, file, lineNo,
				"persistence model or ORM concept leaks through a public/API boundary", core.ConfidenceHigh))
		}
		if domainPath && configLeakPattern.MatchString(trimmed) {
			findings = append(findings, designFinding(env, ruleConfigurationLeak, file, lineNo,
				"configuration or environment concern leaks into domain code", core.ConfidenceMedium))
		}
	}
	if handlerPath {
		findings = append(findings, domainLogicHandlerFinding(env, file, lines)...)
	}
	return firstFindingPerRule(findings)
}

func allowedTypeScriptRecordUtilityLine(line string) bool {
	return strings.Contains(line, "Record<") &&
		!strings.Contains(line, "PrismaClient") &&
		!strings.Contains(line, "Model") &&
		!strings.Contains(line, "Entity") &&
		!strings.Contains(line, "Row")
}

func allowedGeneratedPersistenceEnumLine(line string) bool {
	lowered := strings.ToLower(line)
	if !strings.Contains(lowered, "from") || !strings.Contains(lowered, "@prisma/client") {
		return false
	}
	if strings.Contains(line, "PrismaClient") || strings.Contains(line, "Prisma.") {
		return false
	}
	open := strings.Index(line, "{")
	close := strings.Index(line, "}")
	if open < 0 || close <= open {
		return false
	}
	for _, part := range strings.Split(line[open+1:close], ",") {
		name := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(part), "type "))
		if name == "" {
			continue
		}
		if strings.Contains(strings.ToLower(name), " as ") {
			name = strings.TrimSpace(strings.SplitN(name, " as ", 2)[0])
		}
		if !looksLikeGeneratedEnumType(name) {
			return false
		}
	}
	return true
}

func looksLikeGeneratedEnumType(name string) bool {
	if strings.Contains(name, "Client") || strings.Contains(name, "Model") || strings.Contains(name, "Record") ||
		strings.Contains(name, "Row") || strings.Contains(name, "Entity") {
		return false
	}
	if name == "" {
		return false
	}
	first := rune(name[0])
	return first >= 'A' && first <= 'Z'
}

func domainLogicHandlerFinding(env support.Context, file string, lines []string) []core.Finding {
	score := 0
	lineNo := 1
	for idx, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "if ") || strings.Contains(trimmed, " if ") || strings.Contains(trimmed, " switch ") || strings.Contains(trimmed, " for ") {
			if domainTermPattern.MatchString(trimmed) {
				score += 2
				lineNo = idx + 1
			}
		}
		if domainTermPattern.MatchString(trimmed) && mutationPattern.MatchString(trimmed) {
			score++
			lineNo = idx + 1
		}
	}
	if score < domainLogicStatementLimit(env) {
		return nil
	}
	return []core.Finding{designFinding(env, ruleDomainLogicInHandler, file, lineNo,
		"handler/controller contains business-rule branching and mutation instead of delegating to domain services", core.ConfidenceMedium)}
}

func functionAbstractionFindings(env support.Context, file string, fn designFunction) []core.Finding {
	findings := make([]core.Finding, 0, 2)
	if isPassThroughFunction(fn) {
		findings = append(findings, designFinding(env, rulePassThrough, file, fn.StartLine,
			fmt.Sprintf("function %s mostly passes through to another dependency without policy, validation, or translation", fn.Name), core.ConfidenceMedium))
	}
	if hasTemporalCoupling(fn) {
		findings = append(findings, designFinding(env, ruleTemporalCoupling, file, fn.StartLine,
			fmt.Sprintf("function %s relies on an implicit setup-before-action call order", fn.Name), core.ConfidenceLow))
	}
	return findings
}

func isPassThroughFunction(fn designFunction) bool {
	if len(nonEmptyStatements(fn.Statements)) > 2 {
		return false
	}
	for _, stmt := range fn.Statements {
		line := strings.TrimSpace(stmt.Text)
		if strings.HasPrefix(line, "return ") && delegationCallPattern.MatchString(line) {
			return true
		}
		if delegationCallPattern.MatchString(line) && !strings.Contains(line, " if ") && !strings.Contains(line, "for ") {
			return true
		}
	}
	return false
}

func domainLogicStatementLimit(env support.Context) int {
	threshold := env.Config.Checks.DesignRules.MaxInterfaceMethods / 2
	return max(2, threshold)
}

func hasTemporalCoupling(fn designFunction) bool {
	seenSetup := false
	for _, call := range fn.Calls {
		if temporalSetupPattern.MatchString(call.Callee) {
			seenSetup = true
			continue
		}
		if seenSetup && temporalActionPattern.MatchString(call.Callee) {
			return true
		}
	}
	return false
}

func designFunctions(env support.Context, language string, file string, data []byte) []designFunction {
	switch support.NormalizedLanguage(language) {
	case "", "go":
		return goDesignFunctions(env, file, data)
	case "python", "py":
		return parsedDesignFunctions(support.ParsePython(string(data)))
	case "typescript", "javascript", "ts", "tsx", "js", "jsx":
		return parsedDesignFunctions(support.ParseCLike(string(data), support.CLikeTypeScript))
	case "c++", "cpp", "cxx", "cc":
		return parsedDesignFunctions(support.ParseCLike(string(data), support.CLikeCPP))
	default:
		return nil
	}
}

func goDesignFunctions(env support.Context, file string, data []byte) []designFunction {
	fset, parsed, err := support.ParseGoSource(env, file, data)
	if err != nil {
		return nil
	}
	functions := make([]designFunction, 0)
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		functions = append(functions, goDesignFunction(fset, fn, data))
	}
	return functions
}

func goDesignFunction(fset *token.FileSet, fn *ast.FuncDecl, data []byte) designFunction {
	out := designFunction{Name: fn.Name.Name, StartLine: fset.Position(fn.Pos()).Line}
	start := fset.Position(fn.Body.Lbrace).Offset
	end := fset.Position(fn.Body.Rbrace).Offset
	if start >= 0 && end > start && end <= len(data) {
		for idx, line := range strings.Split(string(data[start+1:end]), "\n") {
			if strings.TrimSpace(line) != "" {
				out.Statements = append(out.Statements, support.ParsedStatement{Line: fset.Position(fn.Body.Lbrace).Line + idx, Text: line, Raw: line})
			}
		}
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			out.Calls = append(out.Calls, support.ParsedCall{Callee: goDesignCallName(call.Fun), Line: fset.Position(call.Pos()).Line})
		}
		return true
	})
	return out
}

func goDesignCallName(expr ast.Expr) string {
	switch value := expr.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.SelectorExpr:
		prefix := goDesignCallName(value.X)
		if prefix == "" {
			return value.Sel.Name
		}
		return prefix + "." + value.Sel.Name
	default:
		var buf bytes.Buffer
		_ = printer.Fprint(&buf, token.NewFileSet(), expr)
		return buf.String()
	}
}

func parsedDesignFunctions(parsed *support.ParsedFile) []designFunction {
	functions := parsed.AllFunctions()
	out := make([]designFunction, 0, len(functions))
	for _, fn := range functions {
		out = append(out, designFunction{Name: fn.Name, StartLine: fn.StartLine, Statements: fn.Statements, Calls: fn.Calls})
	}
	return out
}

func publicSymbols(language string, file string, source string) []publicSymbol {
	switch support.NormalizedLanguage(language) {
	case "", "go":
		return goPublicSymbols(source)
	case "python", "py":
		return regexPublicSymbols(source, pythonPublicPattern)
	case "typescript", "javascript", "ts", "tsx", "js", "jsx":
		return regexPublicSymbols(source, tsExportPattern)
	case "c++", "cpp", "cxx", "cc":
		if !strings.HasSuffix(file, ".h") && !strings.HasSuffix(file, ".hh") && !strings.HasSuffix(file, ".hpp") {
			return nil
		}
		return regexPublicSymbols(source, cppPublicPattern)
	default:
		return nil
	}
}

func goPublicSymbols(source string) []publicSymbol {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, "source.go", source, 0)
	if err != nil {
		return nil
	}
	symbols := make([]publicSymbol, 0)
	for _, decl := range parsed.Decls {
		switch node := decl.(type) {
		case *ast.FuncDecl:
			if node.Name.IsExported() {
				symbols = append(symbols, publicSymbol{Name: node.Name.Name, Line: fset.Position(node.Pos()).Line})
			}
		case *ast.GenDecl:
			for _, spec := range node.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					if spec.Name.IsExported() {
						symbols = append(symbols, publicSymbol{Name: spec.Name.Name, Line: fset.Position(spec.Pos()).Line})
					}
				case *ast.ValueSpec:
					for _, name := range spec.Names {
						if name.IsExported() {
							symbols = append(symbols, publicSymbol{Name: name.Name, Line: fset.Position(name.Pos()).Line})
						}
					}
				}
			}
		}
	}
	return symbols
}

func regexPublicSymbols(source string, pattern *regexp.Regexp) []publicSymbol {
	matches := pattern.FindAllStringSubmatchIndex(source, -1)
	symbols := make([]publicSymbol, 0, len(matches))
	for _, match := range matches {
		name := ""
		for idx := 2; idx+1 < len(match); idx += 2 {
			if match[idx] >= 0 {
				name = source[match[idx]:match[idx+1]]
				break
			}
		}
		if name != "" && !strings.HasPrefix(name, "_") {
			symbols = append(symbols, publicSymbol{Name: name, Line: support.LineNumberForOffset(source, match[0])})
		}
	}
	return symbols
}

func exportedWrapperDensity(source string) int {
	count := 0
	for _, line := range strings.Split(source, "\n") {
		if strings.Contains(line, "return ") && delegationCallPattern.MatchString(line) {
			count++
		}
	}
	return count
}

func averageFunctionStatements(functions []designFunction) int {
	if len(functions) == 0 {
		return 0
	}
	total := 0
	for _, fn := range functions {
		total += len(nonEmptyStatements(fn.Statements))
	}
	return total / len(functions)
}

func nonEmptyStatements(statements []support.ParsedStatement) []support.ParsedStatement {
	out := make([]support.ParsedStatement, 0, len(statements))
	for _, statement := range statements {
		if strings.TrimSpace(statement.Text) != "" {
			out = append(out, statement)
		}
	}
	return out
}

func isDomainPath(file string) bool {
	normalized := strings.ToLower(filepathSlash(file))
	return strings.Contains(normalized, "/domain/") || strings.Contains(normalized, "/core/") ||
		strings.Contains(normalized, "/model/") || strings.Contains(normalized, "/models/")
}

func isAPIPath(file string) bool {
	normalized := strings.ToLower(filepathSlash(file))
	return strings.Contains(normalized, "/api/") || strings.Contains(normalized, "/contract/") ||
		strings.Contains(normalized, "/contracts/")
}

func isHandlerPath(file string) bool {
	normalized := strings.ToLower(filepathSlash(file))
	return strings.Contains(normalized, "handler") || strings.Contains(normalized, "controller") ||
		strings.Contains(normalized, "/routes/") || strings.Contains(normalized, "/views/")
}

func filepathSlash(path string) string {
	return strings.ReplaceAll(path, "\\", "/")
}

func isPublicDeclaration(line string) bool {
	return strings.HasPrefix(line, "export ") || strings.HasPrefix(line, "public ") ||
		strings.HasPrefix(line, "func ") || strings.HasPrefix(line, "type ") ||
		strings.HasPrefix(line, "class ") || strings.HasPrefix(line, "def ")
}

func firstFindingPerRule(findings []core.Finding) []core.Finding {
	seen := map[string]struct{}{}
	out := make([]core.Finding, 0, len(findings))
	for _, finding := range findings {
		if _, ok := seen[finding.RuleID]; ok {
			continue
		}
		seen[finding.RuleID] = struct{}{}
		out = append(out, finding)
	}
	return out
}

func designFinding(env support.Context, ruleID string, file string, line int, message string, confidence string) core.Finding {
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
