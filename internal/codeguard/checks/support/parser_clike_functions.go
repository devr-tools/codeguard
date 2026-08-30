package support

import (
	"regexp"
	"sort"
	"strings"
)

type cppClassSpan struct {
	name     string
	bodyOpen int
	bodyEnd  int
}

type cppNamespaceSpan struct {
	name     string
	bodyOpen int
	bodyEnd  int
}

var (
	cppClassHeadPattern   = regexp.MustCompile(`\b(?:class|struct)\s+([A-Za-z_]\w*)[^;{}]*\{`)
	cppNamespacePattern   = regexp.MustCompile(`\bnamespace\s+([A-Za-z_]\w*(?:::[A-Za-z_]\w*)*)\s*\{`)
	cppDeclarationPattern = regexp.MustCompile(`(?m)(?:^|[;{}])[ \t]*((?:(?:constexpr|constinit|consteval|static|inline|const|volatile|mutable|thread_local|extern)[ \t]+)*(?:[A-Za-z_]\w*(?:::[A-Za-z_]\w*)*)(?:[ \t]*<[^;\n{}=()]+>)?(?:[ \t]*[*&]+[ \t]*|[ \t]+))([A-Za-z_]\w*)[ \t]*(?:=[ \t]*([^;\n]*)|\{([^;\n]*)\}|;)`)
	cppLambdaPattern      = regexp.MustCompile(`\[([^]\n]*)\][ \t]*(?:\([^)]*\)[ \t]*)?(?:mutable[ \t]*)?(?:noexcept[ \t]*)?\{`)
	cppSimpleAliasPattern = regexp.MustCompile(`^\s*(?:std::move\s*\(\s*)?&?([A-Za-z_]\w*)\s*\)?\s*$`)
)

func populateCPPDeclarationMetadata(file *ParsedFile, spans []clikeSpan) {
	namespaces := cppNamespaceSpans(file.Masked)
	classes := cppClassSpans(file.Masked, namespaces)
	maskedTop := []byte(file.Masked)
	for _, span := range spans {
		blankCPPRange(maskedTop, span.start, span.bodyEnd+1)
	}

	declarations := make([]ParsedDeclaration, 0)
	for _, class := range classes {
		classMasked := []byte(file.Masked[class.bodyOpen+1 : class.bodyEnd])
		for _, span := range spans {
			if span.start > class.bodyOpen && span.bodyEnd < class.bodyEnd {
				blankCPPRange(classMasked, span.start-(class.bodyOpen+1), span.bodyEnd+1-(class.bodyOpen+1))
			}
		}
		members := cppDeclarationsInRange(file, string(classMasked), class.bodyOpen+1, "member", class.name, class.bodyOpen, class.bodyEnd)
		declarations = append(declarations, members...)
		blankCPPRange(maskedTop, class.bodyOpen, class.bodyEnd+1)
	}
	globals := cppDeclarationsInRange(file, string(maskedTop), 0, "global", "", 0, len(file.Masked)-1)
	declarations = append(declarations, globals...)
	file.Declarations = declarations

	for _, fn := range file.AllFunctions() {
		owner := cppQualifiedOwner(fn.Name)
		if owner == "" {
			owner = cppOwnerAt(classes, fn.sourceStart)
		} else if namespace := cppNamespaceOwnerAt(namespaces, fn.sourceStart); namespace != "" && !strings.HasPrefix(owner, namespace+"::") {
			owner = namespace + "::" + owner
		}
		fn.QualifiedOwner = owner
		fn.Declarations = append(fn.Declarations, cppParameterDeclarations(fn)...)
		if fn.bodyOpen >= 0 && fn.bodyEnd > fn.bodyOpen {
			body := file.Masked[fn.bodyOpen+1 : fn.bodyEnd]
			fn.Declarations = append(fn.Declarations, cppDeclarationsInRange(file, body, fn.bodyOpen+1, "local", "", fn.bodyOpen, fn.bodyEnd)...)
			fn.Declarations = append(fn.Declarations, cppLambdaCaptures(file, fn, body)...)
		}
		for _, declaration := range declarations {
			if declaration.Kind == "global" || (declaration.Kind == "member" && declaration.QualifiedOwner == owner) {
				fn.Declarations = append(fn.Declarations, declaration)
			}
		}
		sort.SliceStable(fn.Declarations, func(i, j int) bool {
			if fn.Declarations[i].Line == fn.Declarations[j].Line {
				return cppDeclarationPriority(fn.Declarations[i].Kind) < cppDeclarationPriority(fn.Declarations[j].Kind)
			}
			return fn.Declarations[i].Line < fn.Declarations[j].Line
		})
	}
}

func cppClassSpans(masked string, namespaces []cppNamespaceSpan) []cppClassSpan {
	classes := make([]cppClassSpan, 0)
	for _, match := range cppClassHeadPattern.FindAllStringSubmatchIndex(masked, -1) {
		open := match[1] - 1
		end := matchBracketOffset(masked, open)
		if end > open {
			name := masked[match[2]:match[3]]
			if namespace := cppNamespaceOwnerAt(namespaces, match[0]); namespace != "" {
				name = namespace + "::" + name
			}
			classes = append(classes, cppClassSpan{name: name, bodyOpen: open, bodyEnd: end})
		}
	}
	return classes
}

func cppNamespaceSpans(masked string) []cppNamespaceSpan {
	namespaces := make([]cppNamespaceSpan, 0)
	for _, match := range cppNamespacePattern.FindAllStringSubmatchIndex(masked, -1) {
		open := match[1] - 1
		end := matchBracketOffset(masked, open)
		if end > open {
			namespaces = append(namespaces, cppNamespaceSpan{name: masked[match[2]:match[3]], bodyOpen: open, bodyEnd: end})
		}
	}
	for i := range namespaces {
		parent := ""
		width := int(^uint(0) >> 1)
		for j := range namespaces {
			if i == j || namespaces[i].bodyOpen <= namespaces[j].bodyOpen || namespaces[i].bodyEnd >= namespaces[j].bodyEnd {
				continue
			}
			candidateWidth := namespaces[j].bodyEnd - namespaces[j].bodyOpen
			if candidateWidth < width {
				parent, width = namespaces[j].name, candidateWidth
			}
		}
		if parent != "" && !strings.HasPrefix(namespaces[i].name, parent+"::") {
			namespaces[i].name = parent + "::" + namespaces[i].name
		}
	}
	return namespaces
}

func cppNamespaceOwnerAt(namespaces []cppNamespaceSpan, offset int) string {
	owner := ""
	width := int(^uint(0) >> 1)
	for _, namespace := range namespaces {
		if offset > namespace.bodyOpen && offset < namespace.bodyEnd && namespace.bodyEnd-namespace.bodyOpen < width {
			owner, width = namespace.name, namespace.bodyEnd-namespace.bodyOpen
		}
	}
	return owner
}

func cppDeclarationsInRange(file *ParsedFile, masked string, base int, kind string, owner string, scopeOpen int, scopeEnd int) []ParsedDeclaration {
	declarations := make([]ParsedDeclaration, 0)
	for _, match := range cppDeclarationPattern.FindAllStringSubmatchIndex(masked, -1) {
		typ := strings.TrimSpace(masked[match[2]:match[3]])
		name := masked[match[4]:match[5]]
		if cppDeclarationKeyword(typ) || isCLikeKeyword(name) {
			continue
		}
		initializer := ""
		if match[6] >= 0 {
			initializer = strings.TrimSpace(masked[match[6]:match[7]])
		} else if match[8] >= 0 {
			initializer = "{" + strings.TrimSpace(masked[match[8]:match[9]]) + "}"
		}
		offset := base + match[4]
		start, end := cppLexicalScope(file.Masked, scopeOpen, scopeEnd, offset)
		declarations = append(declarations, ParsedDeclaration{
			Name: name, Type: typ, Kind: kind, ReferenceShape: cppReferenceShape(typ),
			Line: LineNumberForOffset(file.Source, offset), ScopeStart: LineNumberForOffset(file.Source, start), ScopeEnd: LineNumberForOffset(file.Source, end),
			AliasSource: cppAliasSource(initializer), Initializer: initializer, QualifiedOwner: owner,
		})
	}
	return declarations
}

func cppParameterDeclarations(fn *ParsedFunction) []ParsedDeclaration {
	out := make([]ParsedDeclaration, 0, len(fn.Params))
	for _, param := range fn.Params {
		if param.Name == "" {
			continue
		}
		out = append(out, ParsedDeclaration{
			Name: param.Name, Type: param.Type, Kind: "parameter", ReferenceShape: cppReferenceShape(param.Type),
			Line: fn.StartLine, ScopeStart: fn.StartLine, ScopeEnd: fn.EndLine,
		})
	}
	return out
}

func cppLambdaCaptures(file *ParsedFile, fn *ParsedFunction, body string) []ParsedDeclaration {
	out := make([]ParsedDeclaration, 0)
	for _, match := range cppLambdaPattern.FindAllStringSubmatchIndex(body, -1) {
		captureText := body[match[2]:match[3]]
		open := fn.bodyOpen + 1 + match[1] - 1
		end := matchBracketOffset(file.Masked, open)
		if end < open {
			continue
		}
		line := LineNumberForOffset(file.Source, fn.bodyOpen+1+match[0])
		for _, raw := range splitTopLevelArgs(captureText) {
			capture := strings.TrimSpace(raw)
			if capture == "" {
				continue
			}
			if capture == "=" || capture == "&" {
				shape := "value"
				if capture == "&" {
					shape = "reference"
				}
				out = append(out, ParsedDeclaration{Name: "*", Kind: "capture", ReferenceShape: shape, Line: line, ScopeStart: line, ScopeEnd: LineNumberForOffset(file.Source, end)})
				continue
			}
			shape := "value"
			capture = strings.TrimSpace(capture)
			if strings.HasPrefix(capture, "&") {
				shape = "reference"
				capture = strings.TrimSpace(strings.TrimPrefix(capture, "&"))
			}
			name := capture
			initializer := ""
			if eq := topLevelIndex(capture, '='); eq >= 0 {
				name = strings.TrimSpace(capture[:eq])
				initializer = strings.TrimSpace(capture[eq+1:])
			}
			name = strings.TrimPrefix(name, "*")
			if !clikeIdentPattern.MatchString(name) && name != "this" {
				continue
			}
			out = append(out, ParsedDeclaration{Name: name, Kind: "capture", ReferenceShape: shape, AliasSource: cppFirstNonEmpty(initializer, name), Initializer: initializer, Line: line, ScopeStart: line, ScopeEnd: LineNumberForOffset(file.Source, end)})
		}
	}
	return out
}

func cppLexicalScope(masked string, outerOpen int, outerEnd int, offset int) (int, int) {
	start, end := outerOpen, outerEnd
	stack := make([]int, 0)
	for i := outerOpen + 1; i < offset && i < outerEnd; i++ {
		switch masked[i] {
		case '{':
			stack = append(stack, i)
		case '}':
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	if len(stack) > 0 {
		start = stack[len(stack)-1]
		if scopeEnd := matchBracketOffset(masked, start); scopeEnd > start {
			end = scopeEnd
		}
	}
	return start, end
}

func cppReferenceShape(typ string) string {
	if strings.Contains(typ, "&") {
		return "reference"
	}
	if strings.Contains(typ, "*") {
		return "pointer"
	}
	return ""
}

func cppAliasSource(initializer string) string {
	if match := cppSimpleAliasPattern.FindStringSubmatch(strings.TrimSpace(initializer)); len(match) == 2 {
		return match[1]
	}
	return ""
}

func cppQualifiedOwner(name string) string {
	if cut := strings.LastIndex(name, "::"); cut > 0 {
		return name[:cut]
	}
	return ""
}

func cppOwnerAt(classes []cppClassSpan, offset int) string {
	owner := ""
	width := int(^uint(0) >> 1)
	for _, class := range classes {
		if offset > class.bodyOpen && offset < class.bodyEnd && class.bodyEnd-class.bodyOpen < width {
			owner, width = class.name, class.bodyEnd-class.bodyOpen
		}
	}
	return owner
}

func cppDeclarationKeyword(typ string) bool {
	base := strings.Fields(strings.TrimSpace(typ))
	if len(base) == 0 {
		return true
	}
	switch base[len(base)-1] {
	case "return", "if", "else", "for", "while", "switch", "case", "catch", "throw", "delete", "new", "template", "typename", "class", "struct", "using", "namespace":
		return true
	default:
		return false
	}
}

func cppDeclarationPriority(kind string) int {
	switch kind {
	case "capture":
		return 0
	case "local":
		return 1
	case "parameter":
		return 2
	case "member":
		return 3
	default:
		return 4
	}
}

func blankCPPRange(data []byte, start int, end int) {
	if start < 0 {
		start = 0
	}
	if end > len(data) {
		end = len(data)
	}
	for i := start; i < end; i++ {
		if data[i] != '\n' {
			data[i] = ' '
		}
	}
}

func cppFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
