package quality

import (
	"context"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	pythonFunctionPattern   = regexp.MustCompile(`^\s*(?:async\s+def|def)\s+([A-Za-z_]\w*)\s*\((.*)\)\s*:`)
	tsFunctionPattern       = regexp.MustCompile(`^\s*(?:export\s+)?(?:async\s+)?function\s+([A-Za-z_$][\w$]*)\s*(?:<[^>]+>)?\s*\(([^)]*)\)`)
	tsArrowPattern          = regexp.MustCompile(`^\s*(?:export\s+)?(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*(?:async\s*)?\(([^)]*)\)\s*(?::[^=]+)?=>`)
	tsMethodPattern         = regexp.MustCompile(`^\s*(?:public|private|protected|static|readonly|async|\s)*([A-Za-z_$][\w$]*)\s*\(([^)]*)\)\s*(?::[^{]+)?\{`)
	tsExplicitAnyPattern    = regexp.MustCompile(`(?:[:<,(]\s*any\b|\bas\s+any\b)`)
	tsDoubleAssertPattern   = regexp.MustCompile(`\bas\s+(?:unknown|any)\s+as\s+`)
	tsIgnoreCommentPattern  = regexp.MustCompile(`^\s*(?://|/\*+|\*)\s*@ts-ignore\b`)
	tsNoCheckCommentPattern = regexp.MustCompile(`^\s*(?://|/\*+|\*)\s*@ts-nocheck\b`)
)

type functionMetrics struct {
	Name       string
	StartLine  int
	Length     int
	Params     int
	Complexity int
}

func Run(ctx context.Context, env support.Context) core.SectionResult {
	findings := make([]core.Finding, 0)
	for _, target := range env.Config.Targets {
		switch normalizedLanguage(target.Language) {
		case "", "go":
			findings = append(findings, env.ScanTargetFiles(target, "quality", func(rel string) bool {
				return strings.HasSuffix(rel, ".go")
			}, func(file string, data []byte) []core.Finding {
				return goFindingsForFile(env, file, data)
			})...)
		case "python", "py":
			findings = append(findings, env.ScanTargetFiles(target, "quality", func(rel string) bool {
				return strings.HasSuffix(strings.ToLower(rel), ".py")
			}, func(file string, data []byte) []core.Finding {
				return pythonFindingsForFile(env, file, data)
			})...)
		case "typescript", "javascript", "ts", "tsx", "js", "jsx":
			findings = append(findings, env.ScanTargetFiles(target, "quality", isTypeScriptLikeFile, func(file string, data []byte) []core.Finding {
				return typeScriptFindingsForFile(env, file, data)
			})...)
		}
		findings = append(findings, commandFindings(ctx, env, target)...)
	}
	return env.FinalizeSection("quality", "Code Quality", findings)
}

func commandFindings(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	checks := env.Config.Checks.QualityRules.LanguageCommands[normalizedLanguage(target.Language)]
	findings := make([]core.Finding, 0, len(checks))
	for _, check := range checks {
		output, err := env.RunCommandCheck(ctx, target.Path, check)
		if err == nil {
			continue
		}
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "quality.command-check",
			Level:   "fail",
			Message: commandFailureMessage(target, check, output, err),
		}))
	}
	return findings
}

func commandFailureMessage(target core.TargetConfig, check core.CommandCheckConfig, output string, err error) string {
	message := fmt.Sprintf("target %q quality command %q failed", target.Name, check.Name)
	output = trimmedOutput(output)
	if output != "" {
		message += ": " + output
	} else if err != nil {
		message += ": " + err.Error()
	}
	return message
}

func goFindingsForFile(env support.Context, file string, data []byte) []core.Finding {
	findings := fileLengthFinding(env, file, data)

	formatted, err := format.Source(data)
	if err != nil {
		return append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "quality.parse-error",
			Level:   "fail",
			Path:    file,
			Line:    1,
			Column:  1,
			Message: fmt.Sprintf("Go parse error: %v", err),
		}))
	}
	if string(formatted) != string(data) {
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "quality.gofmt",
			Level:   "fail",
			Path:    file,
			Line:    1,
			Column:  1,
			Message: "file is not gofmt-formatted",
		}))
	}

	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file, data, parser.ParseComments)
	if err != nil {
		return append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "quality.parse-error",
			Level:   "fail",
			Path:    file,
			Line:    1,
			Column:  1,
			Message: fmt.Sprintf("Go parse error: %v", err),
		}))
	}
	if len(parsed.Decls) > env.Config.Checks.DesignRules.MaxDeclsPerFile {
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "design.max-decls-per-file",
			Level:   "warn",
			Path:    file,
			Line:    1,
			Column:  1,
			Message: fmt.Sprintf("file has %d declarations; max is %d", len(parsed.Decls), env.Config.Checks.DesignRules.MaxDeclsPerFile),
		}))
	}
	findings = append(findings, importFindings(env, file, fset, parsed)...)
	findings = append(findings, goFunctionFindings(env, file, fset, parsed)...)
	return findings
}

func pythonFindingsForFile(env support.Context, file string, data []byte) []core.Finding {
	findings := fileLengthFinding(env, file, data)
	for _, fn := range pythonFunctions(string(data)) {
		findings = append(findings, maintainabilityFindings(env, file, fn)...)
	}
	return findings
}

func typeScriptFindingsForFile(env support.Context, file string, data []byte) []core.Finding {
	findings := fileLengthFinding(env, file, data)
	source := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(source, "\n")
	code := stripTypeScriptCommentsAndStrings(source)
	for idx, line := range lines {
		switch {
		case tsIgnoreCommentPattern.MatchString(line):
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID:  "quality.typescript.ts-ignore",
				Level:   "warn",
				Path:    file,
				Line:    idx + 1,
				Column:  1,
				Message: "TypeScript suppression comment should be reviewed",
			}))
		case tsNoCheckCommentPattern.MatchString(line):
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID:  "quality.typescript.ts-nocheck",
				Level:   "warn",
				Path:    file,
				Line:    idx + 1,
				Column:  1,
				Message: "TypeScript file-level type checking is disabled",
			}))
		}
	}
	findings = append(findings, typeScriptPatternFindings(env, file, source, code)...)
	for _, fn := range typeScriptFunctions(string(data)) {
		findings = append(findings, maintainabilityFindings(env, file, fn)...)
	}
	return findings
}

func typeScriptPatternFindings(env support.Context, file string, source string, code string) []core.Finding {
	findings := make([]core.Finding, 0, 3)
	findings = append(findings, regexTypeScriptFinding(env, file, source, code, tsExplicitAnyPattern, "quality.typescript.explicit-any", "warn", "explicit any should be reviewed")...)
	findings = append(findings, regexTypeScriptFinding(env, file, source, code, tsDoubleAssertPattern, "quality.typescript.double-assertion", "warn", "double type assertions should be reviewed")...)
	for _, line := range typeScriptNonNullAssertionLines(code) {
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "quality.typescript.non-null-assertion",
			Level:   "warn",
			Path:    file,
			Line:    line,
			Column:  1,
			Message: "non-null assertions should be reviewed",
		}))
	}
	return findings
}

func regexTypeScriptFinding(env support.Context, file string, source string, code string, pattern *regexp.Regexp, ruleID string, level string, message string) []core.Finding {
	matches := pattern.FindAllStringIndex(code, -1)
	if len(matches) == 0 {
		return nil
	}
	findings := make([]core.Finding, 0, len(matches))
	seenLines := make(map[int]struct{}, len(matches))
	for _, match := range matches {
		line := lineNumberForOffset(source, match[0])
		if _, exists := seenLines[line]; exists {
			continue
		}
		seenLines[line] = struct{}{}
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  ruleID,
			Level:   level,
			Path:    file,
			Line:    line,
			Column:  1,
			Message: message,
		}))
	}
	return findings
}

func typeScriptNonNullAssertionLines(code string) []int {
	lines := make([]int, 0)
	seen := make(map[int]struct{})
	for idx := 0; idx < len(code); idx++ {
		if code[idx] != '!' {
			continue
		}
		prev := previousSignificantByte(code, idx)
		next := nextSignificantByte(code, idx+1)
		if !isTypeScriptAssertionTarget(prev) {
			continue
		}
		if next == '=' || next == '!' {
			continue
		}
		line := lineNumberForOffset(code, idx)
		if _, exists := seen[line]; exists {
			continue
		}
		seen[line] = struct{}{}
		lines = append(lines, line)
	}
	return lines
}

func fileLengthFinding(env support.Context, file string, data []byte) []core.Finding {
	lineCount := env.CountLines(data)
	if lineCount <= env.Config.Checks.QualityRules.MaxFileLines {
		return nil
	}
	return []core.Finding{env.NewFinding(support.FindingInput{
		RuleID:  "quality.max-file-lines",
		Level:   "warn",
		Path:    file,
		Line:    lineCount,
		Column:  1,
		Message: fmt.Sprintf("file has %d lines; max is %d", lineCount, env.Config.Checks.QualityRules.MaxFileLines),
	})}
}

func maintainabilityFindings(env support.Context, file string, fn functionMetrics) []core.Finding {
	findings := make([]core.Finding, 0, 3)
	if fn.Length > env.Config.Checks.QualityRules.MaxFunctionLines {
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "quality.max-function-lines",
			Level:   "warn",
			Path:    file,
			Line:    fn.StartLine,
			Column:  1,
			Message: fmt.Sprintf("function %s has %d lines; max is %d", fn.Name, fn.Length, env.Config.Checks.QualityRules.MaxFunctionLines),
		}))
	}
	if fn.Params > env.Config.Checks.QualityRules.MaxParameters {
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "quality.max-parameters",
			Level:   "warn",
			Path:    file,
			Line:    fn.StartLine,
			Column:  1,
			Message: fmt.Sprintf("function %s has %d parameters; max is %d", fn.Name, fn.Params, env.Config.Checks.QualityRules.MaxParameters),
		}))
	}
	if fn.Complexity > env.Config.Checks.QualityRules.MaxCyclomaticComplexity {
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "quality.cyclomatic-complexity",
			Level:   "warn",
			Path:    file,
			Line:    fn.StartLine,
			Column:  1,
			Message: fmt.Sprintf("function %s has cyclomatic complexity %d; max is %d", fn.Name, fn.Complexity, env.Config.Checks.QualityRules.MaxCyclomaticComplexity),
		}))
	}
	return findings
}

func importFindings(env support.Context, file string, fset *token.FileSet, parsed *ast.File) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, imp := range parsed.Imports {
		pathValue := strings.Trim(imp.Path.Value, `"`)
		if strings.Contains(pathValue, "/internal/") && allowsInternalImport(env, file) {
			pos := fset.Position(imp.Pos())
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID:  "quality.dependency-direction",
				Level:   "warn",
				Path:    file,
				Line:    pos.Line,
				Column:  pos.Column,
				Message: "non-CLI package imports internal implementation detail",
			}))
		}
	}
	return findings
}

func allowsInternalImport(env support.Context, file string) bool {
	if env.IsInternalOrCmdFile(file) {
		return false
	}
	if strings.HasPrefix(filepath.ToSlash(file), "tests/") {
		return false
	}
	return !env.IsSDKFacadeFile(file)
}

func goFunctionFindings(env support.Context, file string, fset *token.FileSet, parsed *ast.File) []core.Finding {
	findings := make([]core.Finding, 0)
	ast.Inspect(parsed, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}
		start := fset.Position(fn.Pos()).Line
		end := fset.Position(fn.End()).Line
		findings = append(findings, maintainabilityFindings(env, file, functionMetrics{
			Name:       fn.Name.Name,
			StartLine:  start,
			Length:     end - start + 1,
			Params:     countFuncParams(fn),
			Complexity: env.CyclomaticComplexity(fn.Body),
		})...)
		return true
	})
	return findings
}

func countFuncParams(fn *ast.FuncDecl) int {
	if fn.Type == nil || fn.Type.Params == nil {
		return 0
	}
	paramCount := 0
	for _, param := range fn.Type.Params.List {
		if len(param.Names) == 0 {
			paramCount++
			continue
		}
		paramCount += len(param.Names)
	}
	return paramCount
}

func pythonFunctions(source string) []functionMetrics {
	lines := strings.Split(source, "\n")
	functions := make([]functionMetrics, 0)
	for idx, line := range lines {
		match := pythonFunctionPattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		startIndent := indentationWidth(line)
		endIdx := len(lines) - 1
		for j := idx + 1; j < len(lines); j++ {
			trimmed := strings.TrimSpace(lines[j])
			if trimmed == "" {
				continue
			}
			if indentationWidth(lines[j]) <= startIndent {
				endIdx = j - 1
				break
			}
		}
		body := strings.Join(lines[min(idx+1, len(lines)):endIdx+1], "\n")
		functions = append(functions, functionMetrics{
			Name:       match[1],
			StartLine:  idx + 1,
			Length:     max(1, endIdx-idx+1),
			Params:     countParameters(match[2]),
			Complexity: pythonComplexity(body),
		})
	}
	return functions
}

func typeScriptFunctions(source string) []functionMetrics {
	lines := strings.Split(source, "\n")
	functions := make([]functionMetrics, 0)
	for idx, line := range lines {
		name, params, matched := matchedTypeScriptFunction(line)
		if !matched {
			continue
		}
		openIdx := strings.LastIndex(line, "{")
		if openIdx < 0 {
			continue
		}
		endIdx := findBraceBlockEnd(lines, idx, openIdx)
		body := strings.Join(lines[min(idx+1, len(lines)):endIdx+1], "\n")
		functions = append(functions, functionMetrics{
			Name:       name,
			StartLine:  idx + 1,
			Length:     max(1, endIdx-idx+1),
			Params:     countParameters(params),
			Complexity: typeScriptComplexity(body),
		})
	}
	return functions
}

func matchedTypeScriptFunction(line string) (string, string, bool) {
	if match := tsFunctionPattern.FindStringSubmatch(line); match != nil {
		return match[1], match[2], true
	}
	if match := tsArrowPattern.FindStringSubmatch(line); match != nil {
		return match[1], match[2], true
	}
	if match := tsMethodPattern.FindStringSubmatch(line); match != nil {
		name := match[1]
		switch name {
		case "if", "for", "while", "switch", "catch", "constructor":
			return "", "", false
		}
		return match[1], match[2], true
	}
	return "", "", false
}

func findBraceBlockEnd(lines []string, start int, openIdx int) int {
	depth := 0
	for i := start; i < len(lines); i++ {
		line := lines[i]
		startColumn := 0
		if i == start {
			startColumn = openIdx
		}
		for _, ch := range line[startColumn:] {
			switch ch {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					return i
				}
			}
		}
	}
	return len(lines) - 1
}

func countParameters(signature string) int {
	signature = strings.TrimSpace(signature)
	if signature == "" {
		return 0
	}
	parts := strings.Split(signature, ",")
	count := 0
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		count++
	}
	return count
}

func pythonComplexity(body string) int {
	complexity := 1
	for _, pattern := range []string{" if ", " elif ", " for ", " while ", " except ", " case ", " and ", " or "} {
		complexity += strings.Count(" "+body+" ", pattern)
	}
	return complexity
}

func typeScriptComplexity(body string) int {
	complexity := 1
	for _, pattern := range []string{"if (", "for (", "while (", "case ", "catch (", "&&", "||", " ? "} {
		complexity += strings.Count(body, pattern)
	}
	return complexity
}

func isTypeScriptLikeFile(rel string) bool {
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".mts", ".cts":
		return true
	default:
		return false
	}
}

func indentationWidth(line string) int {
	width := 0
	for _, ch := range line {
		if ch == ' ' {
			width++
			continue
		}
		if ch == '\t' {
			width += 4
			continue
		}
		break
	}
	return width
}

func normalizedLanguage(language string) string {
	return strings.ToLower(strings.TrimSpace(language))
}

func trimmedOutput(output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return ""
	}
	output = strings.Join(strings.Fields(output), " ")
	if len(output) > 240 {
		return output[:237] + "..."
	}
	return output
}

func stripTypeScriptCommentsAndStrings(source string) string {
	out := []byte(source)
	state := "code"
	for idx := 0; idx < len(out); idx++ {
		switch state {
		case "code":
			if idx+1 < len(out) && out[idx] == '/' && out[idx+1] == '/' {
				out[idx], out[idx+1] = ' ', ' '
				state = "line-comment"
				idx++
				continue
			}
			if idx+1 < len(out) && out[idx] == '/' && out[idx+1] == '*' {
				out[idx], out[idx+1] = ' ', ' '
				state = "block-comment"
				idx++
				continue
			}
			switch out[idx] {
			case '\'', '"':
				quote := out[idx]
				out[idx] = ' '
				state = string(quote)
			case '`':
				out[idx] = ' '
				state = "template"
			}
		case "line-comment":
			if out[idx] == '\n' {
				state = "code"
				continue
			}
			out[idx] = ' '
		case "block-comment":
			if idx+1 < len(out) && out[idx] == '*' && out[idx+1] == '/' {
				out[idx], out[idx+1] = ' ', ' '
				state = "code"
				idx++
				continue
			}
			if out[idx] != '\n' {
				out[idx] = ' '
			}
		case "'":
			if out[idx] == '\\' && idx+1 < len(out) {
				out[idx], out[idx+1] = ' ', ' '
				idx++
				continue
			}
			if out[idx] == '\n' {
				state = "code"
				continue
			}
			if out[idx] == '\'' {
				state = "code"
			}
			out[idx] = ' '
		case `"`:
			if out[idx] == '\\' && idx+1 < len(out) {
				out[idx], out[idx+1] = ' ', ' '
				idx++
				continue
			}
			if out[idx] == '\n' {
				state = "code"
				continue
			}
			if out[idx] == '"' {
				state = "code"
			}
			out[idx] = ' '
		case "template":
			if out[idx] == '\\' && idx+1 < len(out) {
				if out[idx] != '\n' {
					out[idx] = ' '
				}
				if out[idx+1] != '\n' {
					out[idx+1] = ' '
				}
				idx++
				continue
			}
			if out[idx] == '`' {
				out[idx] = ' '
				state = "code"
				continue
			}
			if out[idx] != '\n' {
				out[idx] = ' '
			}
		}
	}
	return string(out)
}

func lineNumberForOffset(source string, offset int) int {
	if offset <= 0 {
		return 1
	}
	if offset > len(source) {
		offset = len(source)
	}
	return 1 + strings.Count(source[:offset], "\n")
}

func previousSignificantByte(source string, idx int) byte {
	for i := idx - 1; i >= 0; i-- {
		switch source[i] {
		case ' ', '\t', '\n', '\r':
			continue
		default:
			return source[i]
		}
	}
	return 0
}

func nextSignificantByte(source string, idx int) byte {
	for i := idx; i < len(source); i++ {
		switch source[i] {
		case ' ', '\t', '\n', '\r':
			continue
		default:
			return source[i]
		}
	}
	return 0
}

func isTypeScriptAssertionTarget(ch byte) bool {
	switch {
	case ch == ')' || ch == ']' || ch == '}' || ch == '$' || ch == '_':
		return true
	case ch >= '0' && ch <= '9':
		return true
	case ch >= 'A' && ch <= 'Z':
		return true
	case ch >= 'a' && ch <= 'z':
		return true
	default:
		return false
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
