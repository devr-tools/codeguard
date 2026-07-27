package quality

import (
	"fmt"
	"go/ast"
	"go/token"
	"regexp"
	"sort"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

const (
	smellGodObjectRuleID    = "smell.god-object"
	smellFeatureEnvyRuleID  = "smell.feature-envy"
	smellMiddleManRuleID    = "smell.middle-man"
	smellMessageChainRuleID = "smell.message-chain"
	smellDataClumpRuleID    = "smell.data-clump"
	smellSwitchOnTypeRuleID = "smell.switch-on-type"
)

var (
	pythonClassPattern       = regexp.MustCompile(`^(\s*)class\s+([A-Za-z_]\w*)\b`)
	pythonMethodPattern      = regexp.MustCompile(`^(\s*)(?:async\s+)?def\s+([A-Za-z_]\w*)\s*\(([^)]*)\)\s*:`)
	clikeClassPattern        = regexp.MustCompile(`(?m)^[ \t]*(?:export[ \t]+)?(?:default[ \t]+)?(?:class|struct)[ \t]+([A-Za-z_$][\w$]*)[^{;]*\{`)
	clikeMethodLinePattern   = regexp.MustCompile(`^[ \t]*(?:(?:public|private|protected|static|async|virtual|override|inline|constexpr|const|explicit|final)\s+)*(?:[~A-Za-z_$][\w$:<>,*&\s]+\s+)?([~A-Za-z_$][\w$]*)\s*\(([^)]*)\)\s*(?:const\s*)?(?:override\s*)?(?:noexcept\s*)?\{`)
	clikeFieldLinePattern    = regexp.MustCompile(`^[ \t]*(?:(?:public|private|protected|static|readonly|mutable|const|let|var|final)\s+)*(?:[A-Za-z_$][\w$:<>,.?*&\[\]]+\s+)?([A-Za-z_$][\w$]*)\s*(?::[^=;]+)?(?:=[^;]+)?;`)
	delegateReceiverPattern  = regexp.MustCompile(`(?:return\s+)?(?:self|this|[a-zA-Z_]\w*)[.\->]+(_?[A-Za-z_]\w*)[.\->]+[A-Za-z_]\w*\s*\(`)
	delegateLocalPattern     = regexp.MustCompile(`(?:return\s+)?(_?[A-Za-z_]\w*)[.\->]+[A-Za-z_]\w*\s*\(`)
	goKindSwitchPattern      = regexp.MustCompile(`(?m)switch\s+[^{}\n]*(?:\.|_)?(?:kind|type|Kind|Type)\b`)
	pythonKindBranchPattern  = regexp.MustCompile(`(?m)\b(?:if|elif)\s+[^:\n]*(?:\.|_)?(?:kind|type)\b[^:\n]*(?:==| in )`)
	scriptKindSwitchPattern  = regexp.MustCompile(`(?m)switch\s*\([^)]*(?:\.|_)?(?:kind|type|Kind|Type)\b[^)]*\)`)
	cppKindSwitchPattern     = regexp.MustCompile(`(?m)switch\s*\([^)]*(?:\.|_)?(?:kind|type|Kind|Type)\b[^)]*\)`)
	typeBranchPattern        = regexp.MustCompile(`(?m)(?:\.\(type\)|\btypeid\s*\(|\bdynamic_cast\s*<|\binstanceof\b|\btypeof\b|\bisinstance\s*\(|\btype\s*\()`)
	refusedBequestNoopRegexp = regexp.MustCompile(`(?i)\b(unsupported|not\s+implemented|notimplemented|throw\s+new\s+error|raise\s+notimplemented|panic\s*\()`)
)

type structuralClass struct {
	Name      string
	StartLine int
	EndLine   int
	Fields    []string
	Methods   []structuralFunction
}

type structuralFunction struct {
	Name      string
	StartLine int
	EndLine   int
	Owner     string
	Receiver  string
	Params    []support.ParsedParam
	Body      string
}

func goStructuralSmellFindings(env support.Context, file string, fset *token.FileSet, parsed *ast.File, data []byte) []core.Finding {
	classes, functions := goStructuralModel(fset, parsed, data)
	return structuralSmellFindings(env, file, string(data), "go", classes, functions)
}

func parsedStructuralSmellFindings(env support.Context, file string, parsed *support.ParsedFile) []core.Finding {
	classes := sourceStructuralClasses(parsed.Source, parsed.Masked, parsed.Language)
	functions := parsedStructuralFunctions(parsed)
	for _, class := range classes {
		functions = append(functions, class.Methods...)
	}
	return structuralSmellFindings(env, file, parsed.Source, parsed.Language, classes, functions)
}

func structuralSmellFindings(env support.Context, file string, source string, language string, classes []structuralClass, functions []structuralFunction) []core.Finding {
	if isQualityFixturePath(file) {
		return nil
	}
	findings := make([]core.Finding, 0, len(classes)+len(functions)+3)
	findings = append(findings, godObjectFindings(env, file, classes)...)
	findings = append(findings, featureEnvyFindings(env, file, functions)...)
	findings = append(findings, middleManFindings(env, file, classes)...)
	findings = append(findings, messageChainFindings(env, file, source, language)...)
	findings = append(findings, dataClumpFindings(env, file, functions)...)
	findings = append(findings, switchOnTypeFindings(env, file, source, language)...)
	return findings
}

func goStructuralModel(fset *token.FileSet, parsed *ast.File, data []byte) ([]structuralClass, []structuralFunction) {
	classesByName := make(map[string]*structuralClass)
	functions := make([]structuralFunction, 0)
	ast.Inspect(parsed, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.GenDecl:
			for _, spec := range node.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				class := &structuralClass{Name: typeSpec.Name.Name, StartLine: fset.Position(typeSpec.Pos()).Line, EndLine: fset.Position(typeSpec.End()).Line}
				if structType, ok := typeSpec.Type.(*ast.StructType); ok && structType.Fields != nil {
					for _, field := range structType.Fields.List {
						if len(field.Names) == 0 {
							class.Fields = append(class.Fields, goExprText(field.Type))
							continue
						}
						for _, name := range field.Names {
							class.Fields = append(class.Fields, name.Name)
						}
					}
				}
				classesByName[class.Name] = class
			}
		case *ast.FuncDecl:
			fn := goStructuralFunction(fset, node, data)
			functions = append(functions, fn)
			if fn.Owner != "" {
				class := classesByName[fn.Owner]
				if class == nil {
					class = &structuralClass{Name: fn.Owner, StartLine: fn.StartLine, EndLine: fn.EndLine}
					classesByName[fn.Owner] = class
				}
				class.Methods = append(class.Methods, fn)
				if class.StartLine == 0 || fn.StartLine < class.StartLine {
					class.StartLine = fn.StartLine
				}
				if fn.EndLine > class.EndLine {
					class.EndLine = fn.EndLine
				}
			}
		}
		return true
	})
	classes := make([]structuralClass, 0, len(classesByName))
	for _, class := range classesByName {
		classes = append(classes, *class)
	}
	return classes, functions
}

func goStructuralFunction(fset *token.FileSet, fn *ast.FuncDecl, data []byte) structuralFunction {
	out := structuralFunction{
		Name:      fn.Name.Name,
		StartLine: fset.Position(fn.Pos()).Line,
		EndLine:   fset.Position(fn.End()).Line,
		Params:    goParsedParams(fn),
	}
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		recv := fn.Recv.List[0]
		out.Owner = strings.TrimPrefix(strings.TrimPrefix(goExprText(recv.Type), "*"), "[]")
		if len(recv.Names) > 0 {
			out.Receiver = recv.Names[0].Name
		}
	}
	if fn.Body != nil {
		start := fset.Position(fn.Body.Lbrace).Offset
		end := fset.Position(fn.Body.Rbrace).Offset
		if start >= 0 && end > start && end <= len(data) {
			out.Body = string(data[start+1 : end])
		}
	}
	return out
}

func parsedStructuralFunctions(parsed *support.ParsedFile) []structuralFunction {
	parsedFunctions := parsed.AllFunctions()
	functions := make([]structuralFunction, 0, len(parsedFunctions))
	for _, fn := range parsedFunctions {
		functions = append(functions, structuralFunction{
			Name:      fn.Name,
			StartLine: fn.StartLine,
			EndLine:   fn.EndLine,
			Params:    fn.Params,
			Body:      maskedFunctionBody(fn),
		})
	}
	return functions
}

func sourceStructuralClasses(source string, masked string, language string) []structuralClass {
	if language == "python" {
		return pythonStructuralClasses(masked)
	}
	return clikeStructuralClasses(source, masked)
}

func pythonStructuralClasses(masked string) []structuralClass {
	maskedLines := strings.Split(masked, "\n")
	classes := make([]structuralClass, 0)
	for idx := 0; idx < len(maskedLines); idx++ {
		match := pythonClassPattern.FindStringSubmatch(maskedLines[idx])
		if match == nil {
			continue
		}
		classIndent := len(match[1])
		class := structuralClass{Name: match[2], StartLine: idx + 1, EndLine: len(maskedLines)}
		end := len(maskedLines)
		for scan := idx + 1; scan < len(maskedLines); scan++ {
			trimmed := strings.TrimSpace(maskedLines[scan])
			if trimmed == "" {
				continue
			}
			if indentWidthOfLocal(maskedLines[scan]) <= classIndent {
				end = scan
				break
			}
		}
		class.EndLine = end
		for lineIdx := idx + 1; lineIdx < end; lineIdx++ {
			trimmed := strings.TrimSpace(maskedLines[lineIdx])
			if strings.HasPrefix(trimmed, "self.") && strings.Contains(trimmed, "=") {
				class.Fields = append(class.Fields, strings.TrimSpace(strings.SplitN(strings.TrimPrefix(trimmed, "self."), "=", 2)[0]))
			}
			if method := pythonStructuralMethod(maskedLines, lineIdx, end, classIndent, class.Name); method.Name != "" {
				class.Methods = append(class.Methods, method)
			}
		}
		classes = append(classes, class)
		idx = end - 1
	}
	return classes
}

func pythonStructuralMethod(maskedLines []string, lineIdx int, classEnd int, classIndent int, owner string) structuralFunction {
	match := pythonMethodPattern.FindStringSubmatch(maskedLines[lineIdx])
	if match == nil || len(match[1]) <= classIndent {
		return structuralFunction{}
	}
	methodIndent := len(match[1])
	end := classEnd
	for scan := lineIdx + 1; scan < classEnd; scan++ {
		trimmed := strings.TrimSpace(maskedLines[scan])
		if trimmed == "" {
			continue
		}
		if indentWidthOfLocal(maskedLines[scan]) <= methodIndent {
			end = scan
			break
		}
	}
	params := simpleParams(match[3], "python")
	receiver := ""
	if len(params) > 0 && (params[0].Name == "self" || params[0].Name == "cls") {
		receiver = params[0].Name
		params = params[1:]
	}
	return structuralFunction{
		Name:      match[2],
		StartLine: lineIdx + 1,
		EndLine:   end,
		Owner:     owner,
		Receiver:  receiver,
		Params:    params,
		Body:      strings.Join(maskedLines[lineIdx+1:end], "\n"),
	}
}

func clikeStructuralClasses(source string, masked string) []structuralClass {
	classes := make([]structuralClass, 0)
	for _, match := range clikeClassPattern.FindAllStringSubmatchIndex(masked, -1) {
		bodyOpen := match[1] - 1
		bodyEnd := matchBraceLocal(masked, bodyOpen)
		if bodyEnd <= bodyOpen {
			continue
		}
		bodyMasked := masked[bodyOpen+1 : bodyEnd]
		bodySource := source[bodyOpen+1 : bodyEnd]
		startLine := support.LineNumberForOffset(source, match[0])
		class := structuralClass{
			Name:      masked[match[2]:match[3]],
			StartLine: startLine,
			EndLine:   support.LineNumberForOffset(source, bodyEnd),
		}
		class.Fields = clikeClassFields(bodyMasked)
		class.Methods = clikeClassMethods(bodySource, bodyMasked, startLine, class.Name)
		classes = append(classes, class)
	}
	return classes
}

func clikeClassFields(body string) []string {
	fields := make([]string, 0)
	for _, line := range strings.Split(body, "\n") {
		if braceDepthBeforeLine(body, line) > 0 {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.Contains(trimmed, "(") || strings.HasSuffix(trimmed, ":") {
			continue
		}
		if match := clikeFieldLinePattern.FindStringSubmatch(line); match != nil && !isCLikeAccessLabel(match[1]) {
			fields = append(fields, match[1])
		}
	}
	return fields
}

func clikeClassMethods(sourceBody string, maskedBody string, classStartLine int, owner string) []structuralFunction {
	methods := make([]structuralFunction, 0)
	offset := 0
	for offset < len(maskedBody) {
		lineEnd := strings.IndexByte(maskedBody[offset:], '\n')
		if lineEnd < 0 {
			lineEnd = len(maskedBody) - offset
		}
		line := maskedBody[offset : offset+lineEnd]
		if braceDepthAtOffset(maskedBody, offset) == 0 {
			if match := clikeMethodLinePattern.FindStringSubmatchIndex(line); match != nil {
				openInLine := strings.LastIndexByte(line[:match[1]], '{')
				if openInLine >= 0 {
					bodyOpen := offset + openInLine
					bodyEnd := matchBraceLocal(maskedBody, bodyOpen)
					if bodyEnd > bodyOpen {
						params := simpleParams(line[match[4]:match[5]], "clike")
						methods = append(methods, structuralFunction{
							Name:      line[match[2]:match[3]],
							StartLine: classStartLine + strings.Count(maskedBody[:offset], "\n"),
							EndLine:   classStartLine + strings.Count(maskedBody[:bodyEnd], "\n"),
							Owner:     owner,
							Receiver:  "this",
							Params:    params,
							Body:      sourceBody[bodyOpen+1 : bodyEnd],
						})
						offset = bodyEnd + 1
						continue
					}
				}
			}
		}
		offset += lineEnd + 1
	}
	return methods
}

func godObjectFindings(env support.Context, file string, classes []structuralClass) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, class := range classes {
		methods := len(class.Methods)
		fields := len(uniqueStrings(class.Fields))
		responsibilities := classResponsibilities(class)
		if methods >= 8 && (fields >= 5 || responsibilities >= 5 || methods >= 10) {
			findings = append(findings, precisionWarnFinding(env, smellGodObjectRuleID, file, class.StartLine,
				fmt.Sprintf("type %s has %d methods, %d fields, and %d responsibility clusters; split cohesive behavior into smaller collaborators", class.Name, methods, fields, responsibilities),
				core.ConfidenceHigh))
		}
	}
	return findings
}

func classResponsibilities(class structuralClass) int {
	seen := make(map[string]struct{})
	for _, method := range class.Methods {
		if bucket := responsibilityBucket(method.Name); bucket != "" {
			seen[bucket] = struct{}{}
		}
	}
	for _, field := range class.Fields {
		if bucket := responsibilityBucket(field); bucket != "" {
			seen[bucket] = struct{}{}
		}
	}
	return len(seen)
}

func responsibilityBucket(name string) string {
	lowered := strings.ToLower(smellIdentifierWords(name))
	for _, bucket := range []string{"auth", "cache", "delete", "email", "event", "fetch", "find", "load", "notify", "parse", "persist", "render", "report", "save", "search", "send", "sync", "update", "validate"} {
		if strings.Contains(lowered, bucket) {
			return bucket
		}
	}
	return ""
}

func featureEnvyFindings(env support.Context, file string, functions []structuralFunction) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, fn := range functions {
		if len(fn.Params) == 0 || fn.Body == "" {
			continue
		}
		dominantName, dominantCount, totalExternal := dominantExternalAccess(fn)
		if dominantCount < 5 || totalExternal < 5 {
			continue
		}
		ownCount := ownAccessCount(fn)
		if dominantCount >= ownCount+4 && totalExternal >= ownCount+4 {
			findings = append(findings, precisionWarnFinding(env, smellFeatureEnvyRuleID, file, fn.StartLine,
				fmt.Sprintf("function %s accesses collaborator %s %d times versus %d own accesses; move behavior closer to the data owner or pass a richer operation", fn.Name, dominantName, dominantCount, ownCount),
				core.ConfidenceMedium))
		}
	}
	return findings
}

func dominantExternalAccess(fn structuralFunction) (string, int, int) {
	bestName := ""
	bestCount := 0
	total := 0
	for _, param := range fn.Params {
		name := strings.TrimSpace(param.Name)
		if name == "" || name == "_" || name == fn.Receiver {
			continue
		}
		count := accessCount(fn.Body, name)
		total += count
		if count > bestCount {
			bestName = name
			bestCount = count
		}
	}
	return bestName, bestCount, total
}

func ownAccessCount(fn structuralFunction) int {
	count := 0
	for _, own := range []string{fn.Receiver, "self", "this"} {
		if own == "" {
			continue
		}
		count += accessCount(fn.Body, own)
	}
	return count
}

func accessCount(body string, name string) int {
	pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\s*(?:\.|->)`)
	return len(pattern.FindAllStringIndex(body, -1))
}

func middleManFindings(env support.Context, file string, classes []structuralClass) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, class := range classes {
		if len(class.Methods) < 4 {
			continue
		}
		delegates := make(map[string]int)
		delegating := 0
		for _, method := range class.Methods {
			if delegate := delegatedTarget(method.Body); delegate != "" {
				delegating++
				delegates[delegate]++
			}
		}
		target, targetCount := dominantStringCount(delegates)
		if delegating >= 4 && delegating*4 >= len(class.Methods)*3 && targetCount >= 3 {
			findings = append(findings, precisionWarnFinding(env, smellMiddleManRuleID, file, class.StartLine,
				fmt.Sprintf("type %s forwards %d of %d methods, mostly to %s, without visible policy or translation", class.Name, delegating, len(class.Methods), target),
				core.ConfidenceHigh))
		}
	}
	return findings
}

func delegatedTarget(body string) string {
	trimmedLines := make([]string, 0)
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, ";"))
		if line == "" || line == "{" || line == "}" {
			continue
		}
		trimmedLines = append(trimmedLines, line)
	}
	if len(trimmedLines) == 0 || len(trimmedLines) > 3 {
		return ""
	}
	bodyText := strings.Join(trimmedLines, " ")
	if refusedBequestNoopRegexp.MatchString(bodyText) {
		return ""
	}
	for _, pattern := range []*regexp.Regexp{delegateReceiverPattern, delegateLocalPattern} {
		if match := pattern.FindStringSubmatch(bodyText); match != nil {
			return strings.TrimPrefix(match[1], "_")
		}
	}
	return ""
}

func messageChainFindings(env support.Context, file string, source string, language string) []core.Finding {
	masked := maskForStructuralLanguage(source, language)
	for idx, line := range strings.Split(masked, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "import ") || strings.HasPrefix(trimmed, "#include") || strings.HasPrefix(trimmed, "package ") {
			continue
		}
		if chainSeparators(trimmed) >= 4 && !looksLikeAllowedFluentChain(trimmed) {
			return []core.Finding{precisionWarnFinding(env, smellMessageChainRuleID, file, idx+1,
				"long message chain reaches through several collaborators; introduce a named query/helper at the boundary",
				core.ConfidenceMedium)}
		}
	}
	return nil
}

func chainSeparators(line string) int {
	line = strings.ReplaceAll(line, "->", ".")
	line = strings.ReplaceAll(line, "::", ".")
	line = strings.ReplaceAll(line, "?.", ".")
	best := 0
	current := 0
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '.':
			current++
			if current > best {
				best = current
			}
		case ';', ',', '=', '+', '-', '*', '/', '<', '>', '{', '}':
			current = 0
		}
	}
	return best
}

func looksLikeAllowedFluentChain(line string) bool {
	lowered := strings.ToLower(line)
	return strings.Contains(lowered, "builder") || strings.Contains(lowered, ".with") || strings.Contains(lowered, ".set")
}

func dataClumpFindings(env support.Context, file string, functions []structuralFunction) []core.Finding {
	type occurrence struct {
		line int
		fn   string
	}
	groups := make(map[string][]occurrence)
	for _, fn := range functions {
		key := primitiveParamGroup(fn.Params)
		if key == "" {
			continue
		}
		groups[key] = append(groups[key], occurrence{line: fn.StartLine, fn: fn.Name})
	}
	for key, occurrences := range groups {
		if len(occurrences) >= 3 {
			return []core.Finding{precisionWarnFinding(env, smellDataClumpRuleID, file, occurrences[2].line,
				fmt.Sprintf("parameter group [%s] appears in %d functions; extract a value object/options type", key, len(occurrences)),
				core.ConfidenceHigh)}
		}
	}
	return nil
}

func primitiveParamGroup(params []support.ParsedParam) string {
	names := make([]string, 0)
	for _, param := range params {
		name := normalizedParamConcept(param.Name)
		if name == "" {
			continue
		}
		if param.Type != "" && !primitiveTypePattern.MatchString(param.Type) {
			continue
		}
		names = append(names, name)
	}
	names = uniqueStrings(names)
	sort.Strings(names)
	if len(names) < 3 {
		return ""
	}
	return strings.Join(names, ", ")
}

func normalizedParamConcept(name string) string {
	name = strings.Trim(strings.ToLower(name), "_$")
	if name == "" || name == "self" || name == "this" || name == "ctx" || name == "context" {
		return ""
	}
	name = strings.ReplaceAll(name, "_", "")
	name = strings.ReplaceAll(name, "-", "")
	return name
}

func switchOnTypeFindings(env support.Context, file string, source string, language string) []core.Finding {
	masked := maskForStructuralLanguage(source, language)
	typeBranches := len(typeBranchPattern.FindAllStringIndex(masked, -1))
	kindBranches := 0
	switch language {
	case "go":
		kindBranches = len(goKindSwitchPattern.FindAllStringIndex(masked, -1))
	case "python":
		kindBranches = len(pythonKindBranchPattern.FindAllStringIndex(masked, -1))
	case "cpp":
		kindBranches = len(cppKindSwitchPattern.FindAllStringIndex(masked, -1))
	default:
		kindBranches = len(scriptKindSwitchPattern.FindAllStringIndex(masked, -1))
	}
	caseBranches := strings.Count(masked, "case ")
	total := typeBranches + kindBranches
	if total >= 2 || (total >= 1 && caseBranches >= 4) || typeBranches >= 3 {
		return []core.Finding{precisionWarnFinding(env, smellSwitchOnTypeRuleID, file, firstTypeBranchLine(masked),
			fmt.Sprintf("type/kind branching appears %d times with %d case-style branches; prefer polymorphism or a dispatch table", total, caseBranches),
			core.ConfidenceMedium)}
	}
	return nil
}

func firstTypeBranchLine(masked string) int {
	idx := len(masked)
	for _, locs := range [][]int{
		firstMatch(typeBranchPattern, masked),
		firstMatch(goKindSwitchPattern, masked),
		firstMatch(pythonKindBranchPattern, masked),
		firstMatch(scriptKindSwitchPattern, masked),
		firstMatch(cppKindSwitchPattern, masked),
	} {
		if len(locs) == 2 && locs[0] < idx {
			idx = locs[0]
		}
	}
	if idx == len(masked) {
		return 1
	}
	return support.LineNumberForOffset(masked, idx)
}

func firstMatch(pattern *regexp.Regexp, text string) []int {
	return pattern.FindStringIndex(text)
}

func maskForStructuralLanguage(source string, language string) string {
	switch language {
	case "python":
		return support.MaskPythonSource(source)
	case "go":
		return support.MaskCLikeSource(source, support.CLikeGo)
	case "cpp":
		return support.MaskCLikeSource(source, support.CLikeCPP)
	default:
		return support.MaskCLikeSource(source, support.CLikeTypeScript)
	}
}

func simpleParams(paramText string, language string) []support.ParsedParam {
	parts := splitTopLevelStructuralArgs(paramText)
	params := make([]support.ParsedParam, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if eq := strings.Index(part, "="); eq >= 0 {
			part = strings.TrimSpace(part[:eq])
		}
		if language == "python" {
			fields := strings.Split(part, ":")
			params = append(params, support.ParsedParam{Name: strings.TrimSpace(fields[0]), Type: typePart(fields)})
			continue
		}
		if colon := strings.Index(part, ":"); colon >= 0 {
			params = append(params, support.ParsedParam{Name: strings.TrimSpace(strings.TrimPrefix(part[:colon], "...")), Type: strings.TrimSpace(part[colon+1:])})
			continue
		}
		fields := strings.Fields(strings.ReplaceAll(part, "&", " "))
		if len(fields) > 0 {
			params = append(params, support.ParsedParam{Name: strings.Trim(strings.TrimPrefix(fields[len(fields)-1], "*"), "&"), Type: strings.Join(fields[:len(fields)-1], " ")})
		}
	}
	return params
}

func typePart(fields []string) string {
	if len(fields) < 2 {
		return ""
	}
	return strings.TrimSpace(fields[1])
}

func splitTopLevelStructuralArgs(text string) []string {
	parts := make([]string, 0)
	start := 0
	depth := 0
	for idx, r := range text {
		switch r {
		case '(', '[', '{', '<':
			depth++
		case ')', ']', '}', '>':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				parts = append(parts, text[start:idx])
				start = idx + 1
			}
		}
	}
	parts = append(parts, text[start:])
	return parts
}

func indentWidthOfLocal(line string) int {
	width := 0
	for _, r := range line {
		switch r {
		case ' ':
			width++
		case '\t':
			width += 4
		default:
			return width
		}
	}
	return width
}

func matchBraceLocal(text string, open int) int {
	if open < 0 || open >= len(text) || text[open] != '{' {
		return -1
	}
	depth := 0
	for idx := open; idx < len(text); idx++ {
		switch text[idx] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return idx
			}
		}
	}
	return -1
}

func braceDepthAtOffset(text string, offset int) int {
	depth := 0
	for idx := 0; idx < offset && idx < len(text); idx++ {
		switch text[idx] {
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		}
	}
	return depth
}

func braceDepthBeforeLine(body string, line string) int {
	idx := strings.Index(body, line)
	if idx < 0 {
		return 0
	}
	return braceDepthAtOffset(body, idx)
}

func isCLikeAccessLabel(name string) bool {
	return name == "public" || name == "private" || name == "protected"
}

func smellIdentifierWords(name string) string {
	var out strings.Builder
	for idx, r := range name {
		if idx > 0 && r >= 'A' && r <= 'Z' {
			out.WriteByte(' ')
		}
		out.WriteRune(r)
	}
	return out.String()
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func dominantStringCount(values map[string]int) (string, int) {
	bestName := ""
	bestCount := 0
	for name, count := range values {
		if count > bestCount {
			bestName = name
			bestCount = count
		}
	}
	return bestName, bestCount
}
