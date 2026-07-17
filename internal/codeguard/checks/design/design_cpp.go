package design

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	cppTypeDeclarationPattern      = regexp.MustCompile(`^\s*(?:template[ \t]*<[^>\n]+>[ \t]*)*(class|struct)[ \t]+([A-Za-z_]\w*)\b`)
	cppNamespaceDeclarationPattern = regexp.MustCompile(`^\s*(?:export[ \t]+)?namespace[ \t]+([A-Za-z_]\w*(?:::[A-Za-z_]\w*)*)\b`)
	cppAccessSpecifierPattern      = regexp.MustCompile(`^\s*(public|protected|private)\s*:\s*$`)
)

func cppTargetFindings(env support.Context, target core.TargetConfig) []core.Finding {
	return support.ScanCPPFiles(env, target, "design", func(file string, data []byte) []core.Finding {
		return cppDesignFindingsForFile(env, file, data)
	})
}

func cppDesignFindingsForFile(env support.Context, file string, data []byte) []core.Finding {
	findings := cppGenericModuleNameFindings(env, file)
	parsed := support.ParseCLike(string(data), support.CLikeCPP)
	findings = append(findings, cppDeclFindings(env, file, parsed)...)
	surfaces := cppTypeSurfaces(parsed.Source)
	cppRecordOutOfLineMethods(surfaces, parsed.Functions)
	for _, surface := range cppSortedTypeSurfaces(surfaces) {
		if count := len(surface.methods); count > env.Config.Checks.DesignRules.MaxMethodsPerType {
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID: "design.cpp.max-methods-per-type", Level: "warn", Path: file, Line: surface.line, Column: 1,
				Message: fmt.Sprintf("C++ type %s has %d methods in this file; max is %d", surface.name, count, env.Config.Checks.DesignRules.MaxMethodsPerType),
			}))
		}
		if !isCPPContractPath(file) || len(surface.publicMethods) <= env.Config.Checks.DesignRules.MaxInterfaceMethods {
			continue
		}
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID: "design.cpp.max-interface-methods", Level: "warn", Path: file, Line: surface.line, Column: 1,
			Message: fmt.Sprintf("C++ type %s exposes %d public methods in this contract; max is %d", surface.name, len(surface.publicMethods), env.Config.Checks.DesignRules.MaxInterfaceMethods),
		}))
	}
	return findings
}

func cppDeclFindings(env support.Context, file string, parsed *support.ParsedFile) []core.Finding {
	count := cppDeclarationCount(parsed)
	if count <= env.Config.Checks.DesignRules.MaxDeclsPerFile {
		return nil
	}
	return []core.Finding{env.NewFinding(support.FindingInput{
		RuleID:  "design.cpp.max-decls-per-file",
		Level:   "warn",
		Path:    file,
		Line:    1,
		Column:  1,
		Message: fmt.Sprintf("C++ file has %d top-level declarations; max is %d", count, env.Config.Checks.DesignRules.MaxDeclsPerFile),
	})}
}

func cppDeclarationCount(parsed *support.ParsedFile) int {
	if parsed == nil {
		return 0
	}
	count := len(parsed.Functions)
	for _, line := range strings.Split(parsed.Masked, "\n") {
		if cppTypeDeclarationPattern.MatchString(line) {
			count++
		}
	}
	return count
}

func cppGenericModuleNameFindings(env support.Context, file string) []core.Finding {
	name := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	for _, forbidden := range env.Config.Checks.DesignRules.ForbiddenPackageNames {
		if strings.EqualFold(name, forbidden) {
			return []core.Finding{env.NewFinding(support.FindingInput{
				RuleID: "design.cpp.generic-module-name", Level: "warn", Path: file, Line: 1, Column: 1,
				Message: fmt.Sprintf("C++ file name %q is too generic", name),
			})}
		}
	}
	return nil
}

type cppTypeSurface struct {
	name          string
	typeName      string
	line          int
	methods       map[string]struct{}
	publicMethods map[string]struct{}
}

type cppTypeBlock struct {
	surface       *cppTypeSurface
	waiting       bool
	bodyDepth     int
	defaultAccess string
	access        string
}

type cppNamespaceBlock struct {
	name      string
	waiting   bool
	bodyDepth int
}

func cppTypeSurfaces(source string) map[string]*cppTypeSurface {
	source = strings.ReplaceAll(source, "\r\n", "\n")
	lines := strings.Split(support.MaskCLikeSource(source, support.CLikeCPP), "\n")
	surfaces := make(map[string]*cppTypeSurface)
	namespaces := make([]*cppNamespaceBlock, 0)
	types := make([]*cppTypeBlock, 0)
	depth := 0

	for idx, line := range lines {
		lineNo := idx + 1
		namespaces = append(namespaces, newCPPNamespaceBlock(line))
		if block := newCPPTypeBlock(line, lineNo, cppNamespacePrefix(namespaces), surfaces); block != nil {
			types = append(types, block)
		}
		countCPPTypeMembers(types, depth, line)
		depth += braceDelta(line)
		openCPPNamespaceBlocks(namespaces, depth, line)
		openCPPTypeBlocks(types, depth, line)
		countCPPTypeMembers(types, depth, line)
		namespaces = pruneCPPNamespaceBlocks(namespaces, depth, line)
		types = pruneCPPTypeBlocks(types, depth, line)
	}

	return surfaces
}

func newCPPNamespaceBlock(line string) *cppNamespaceBlock {
	match := cppNamespaceDeclarationPattern.FindStringSubmatch(line)
	if len(match) != 2 {
		return nil
	}
	return &cppNamespaceBlock{name: match[1], waiting: true}
}

func newCPPTypeBlock(line string, lineNo int, namespacePrefix string, surfaces map[string]*cppTypeSurface) *cppTypeBlock {
	match := cppTypeDeclarationPattern.FindStringSubmatch(line)
	if len(match) != 3 {
		return nil
	}
	defaultAccess := "private"
	if match[1] == "struct" {
		defaultAccess = "public"
	}
	typeName := match[2]
	qualifiedName := typeName
	if namespacePrefix != "" {
		qualifiedName = namespacePrefix + "::" + qualifiedName
	}
	surface := surfaces[qualifiedName]
	if surface == nil {
		surface = &cppTypeSurface{
			name:          qualifiedName,
			typeName:      typeName,
			line:          lineNo,
			methods:       make(map[string]struct{}),
			publicMethods: make(map[string]struct{}),
		}
		surfaces[qualifiedName] = surface
	} else if surface.line <= 0 {
		surface.line = lineNo
	}
	return &cppTypeBlock{
		surface:       surface,
		waiting:       true,
		defaultAccess: defaultAccess,
		access:        defaultAccess,
	}
}

func cppNamespacePrefix(namespaces []*cppNamespaceBlock) string {
	parts := make([]string, 0, len(namespaces))
	for _, block := range namespaces {
		if block == nil || block.waiting || block.name == "" {
			continue
		}
		parts = append(parts, block.name)
	}
	return strings.Join(parts, "::")
}

func countCPPTypeMembers(types []*cppTypeBlock, depth int, line string) {
	for _, block := range types {
		if block == nil || block.waiting || depth != block.bodyDepth {
			continue
		}
		if match := cppAccessSpecifierPattern.FindStringSubmatch(strings.TrimSpace(line)); len(match) == 2 {
			block.access = match[1]
			continue
		}
		key, ok := cppMethodKey(line, block.surface.typeName)
		if !ok {
			continue
		}
		block.surface.methods[key] = struct{}{}
		if block.access == "public" {
			block.surface.publicMethods[key] = struct{}{}
		}
	}
}

func cppMethodKey(line string, typeName string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "using ") ||
		strings.HasPrefix(trimmed, "typedef ") || strings.HasPrefix(trimmed, "friend ") ||
		strings.HasPrefix(trimmed, "static_assert") || strings.HasPrefix(trimmed, "return ") {
		return "", false
	}
	open := strings.Index(trimmed, "(")
	if open < 0 {
		return "", false
	}
	head := strings.TrimSpace(trimmed[:open])
	if head == "" || strings.Contains(head, "=") {
		return "", false
	}
	name := cppTrailingIdentifier(head)
	if name == "" || cppNonMethodName(name) {
		return "", false
	}
	if name != typeName && name != "~"+typeName && strings.Contains(name, "::") {
		name = name[strings.LastIndex(name, "::")+2:]
	}
	close := cppMatchingParen(trimmed, open)
	if close < 0 {
		return "", false
	}
	trailer := strings.TrimSpace(trimmed[close+1:])
	if strings.HasPrefix(trailer, "->") {
		return "", false
	}
	params := cppSquashWhitespace(trimmed[open+1 : close])
	return name + "(" + params + ")", true
}

func cppTrailingIdentifier(head string) string {
	head = strings.TrimRight(head, " \t*&")
	if head == "" {
		return ""
	}
	start := len(head)
	for start > 0 {
		ch := head[start-1]
		if ch == '_' || ch == '~' || ch == ':' ||
			(ch >= '0' && ch <= '9') ||
			(ch >= 'A' && ch <= 'Z') ||
			(ch >= 'a' && ch <= 'z') {
			start--
			continue
		}
		break
	}
	return head[start:]
}

func cppNonMethodName(name string) bool {
	switch name {
	case "if", "for", "while", "switch", "catch", "return", "requires":
		return true
	default:
		return name == ""
	}
}

func cppMatchingParen(line string, open int) int {
	depth := 0
	for i := open; i < len(line); i++ {
		switch line[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func openCPPNamespaceBlocks(blocks []*cppNamespaceBlock, depth int, line string) {
	if !strings.Contains(line, "{") {
		return
	}
	for _, block := range blocks {
		if block == nil || !block.waiting {
			continue
		}
		block.waiting = false
		block.bodyDepth = depth
	}
}

func openCPPTypeBlocks(blocks []*cppTypeBlock, depth int, line string) {
	if !strings.Contains(line, "{") {
		return
	}
	for _, block := range blocks {
		if block == nil || !block.waiting {
			continue
		}
		block.waiting = false
		block.bodyDepth = depth
		block.access = block.defaultAccess
	}
}

func pruneCPPNamespaceBlocks(blocks []*cppNamespaceBlock, depth int, line string) []*cppNamespaceBlock {
	kept := blocks[:0]
	for _, block := range blocks {
		if block == nil {
			continue
		}
		if block.waiting && strings.Contains(line, ";") && !strings.Contains(line, "{") {
			continue
		}
		if !block.waiting && depth < block.bodyDepth {
			continue
		}
		kept = append(kept, block)
	}
	return kept
}

func pruneCPPTypeBlocks(blocks []*cppTypeBlock, depth int, line string) []*cppTypeBlock {
	kept := blocks[:0]
	for _, block := range blocks {
		if block == nil {
			continue
		}
		if block.waiting && strings.Contains(line, ";") && !strings.Contains(line, "{") {
			continue
		}
		if !block.waiting && depth < block.bodyDepth {
			continue
		}
		kept = append(kept, block)
	}
	return kept
}

func cppRecordOutOfLineMethods(surfaces map[string]*cppTypeSurface, functions []*support.ParsedFunction) {
	for _, function := range functions {
		if function == nil {
			continue
		}
		separator := strings.LastIndex(function.Name, "::")
		if separator <= 0 {
			continue
		}
		typeName := function.Name[:separator]
		methodName := function.Name[separator+2:]
		surface := surfaces[typeName]
		if surface == nil {
			surface = &cppTypeSurface{
				name:          typeName,
				typeName:      typeName[strings.LastIndex(typeName, "::")+2:],
				line:          function.StartLine,
				methods:       make(map[string]struct{}),
				publicMethods: make(map[string]struct{}),
			}
			surfaces[typeName] = surface
		}
		surface.methods[methodName+"("+cppSquashWhitespace(function.Signature)+")"] = struct{}{}
	}
}

func cppSortedTypeSurfaces(surfaces map[string]*cppTypeSurface) []*cppTypeSurface {
	result := make([]*cppTypeSurface, 0, len(surfaces))
	for _, surface := range surfaces {
		result = append(result, surface)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].line != result[j].line {
			return result[i].line < result[j].line
		}
		return result[i].name < result[j].name
	})
	return result
}

func isCPPContractPath(file string) bool {
	rawExt := filepath.Ext(file)
	if rawExt == ".C" {
		return false
	}
	switch strings.ToLower(rawExt) {
	case ".h", ".hh", ".hpp", ".hxx", ".h++", ".ipp", ".tpp", ".inl", ".txx", ".ixx",
		".cppm", ".cxxm", ".ccm", ".c++m", ".mpp", ".mxx", ".inc":
		return true
	default:
		return false
	}
}

func cppSquashWhitespace(text string) string {
	return strings.TrimSpace(strings.Join(strings.Fields(text), " "))
}
