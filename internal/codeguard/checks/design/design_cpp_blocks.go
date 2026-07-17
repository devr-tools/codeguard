package design

import (
	"sort"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
)

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
