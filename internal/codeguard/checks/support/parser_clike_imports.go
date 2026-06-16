package support

import (
	"regexp"
	"strings"
)

var (
	tsImportLinePattern   = regexp.MustCompile(`(?m)^[ \t]*import[ \t][^\n]*`)
	tsRequireLinePattern  = regexp.MustCompile(`(?m)^[ \t]*(?:const|let|var)[ \t]+[^\n]*=[ \t]*require\([^\n]*`)
	tsModulePattern       = regexp.MustCompile(`(?:from[ \t]+|^[ \t]*import[ \t]+|require\()['"]([^'"]+)['"]`)
	tsDefaultBindPattern  = regexp.MustCompile(`(?:import|const|let|var)[ \t]+(?:\*[ \t]+as[ \t]+)?([A-Za-z_$][\w$]*)`)
	tsNamedBindPattern    = regexp.MustCompile(`\{([^}]*)\}`)
	javaImportPattern     = regexp.MustCompile(`(?m)^[ \t]*import[ \t]+(?:static[ \t]+)?([\w.]+(?:\.\*)?)[ \t]*;`)
	rustUsePattern        = regexp.MustCompile(`(?m)^[ \t]*(?:pub(?:\([^)\n]*\))?[ \t]+)?use[ \t]+([^;]+);`)
	rustUseGroupedPattern = regexp.MustCompile(`^(.*)::\{(.*)\}$`)
)

func clikeImports(source string, masked string, lang CLikeLanguage) []ParsedImport {
	switch lang {
	case CLikeJava:
		return javaImports(masked)
	case CLikeRust:
		return rustImports(masked)
	default:
		return typeScriptImports(source, masked)
	}
}

// typeScriptImports finds import statements on the masked source, then reads
// module paths from the identical offsets of the raw source.
func typeScriptImports(source string, masked string) []ParsedImport {
	imports := make([]ParsedImport, 0, 4)
	matches := tsImportLinePattern.FindAllStringIndex(masked, -1)
	matches = append(matches, tsRequireLinePattern.FindAllStringIndex(masked, -1)...)
	for _, match := range matches {
		rawLine := source[match[0]:match[1]]
		line := LineNumberForOffset(source, match[0])
		moduleMatch := tsModulePattern.FindStringSubmatch(rawLine)
		if moduleMatch == nil {
			continue
		}
		imports = append(imports, typeScriptImportBindings(rawLine, moduleMatch[1], line)...)
	}
	return imports
}

func typeScriptImportBindings(rawLine string, module string, line int) []ParsedImport {
	imports := make([]ParsedImport, 0, 2)
	head := rawLine
	if from := strings.Index(rawLine, "from"); from >= 0 {
		head = rawLine[:from]
	}
	if named := tsNamedBindPattern.FindStringSubmatch(head); named != nil {
		for _, part := range strings.Split(named[1], ",") {
			name, alias := splitAsAlias(strings.TrimSpace(part))
			if name == "" && alias == "" {
				continue
			}
			if alias == "" {
				alias = name
			}
			imports = append(imports, ParsedImport{Module: module, Name: name, Alias: alias, Line: line})
		}
	}
	withoutBraces := tsNamedBindPattern.ReplaceAllString(head, "")
	if bind := tsDefaultBindPattern.FindStringSubmatch(withoutBraces); bind != nil {
		imports = append(imports, ParsedImport{Module: module, Alias: bind[1], Line: line})
	}
	if len(imports) == 0 {
		imports = append(imports, ParsedImport{Module: module, Line: line})
	}
	return imports
}

func javaImports(masked string) []ParsedImport {
	imports := make([]ParsedImport, 0, 4)
	for _, match := range javaImportPattern.FindAllStringSubmatchIndex(masked, -1) {
		path := masked[match[2]:match[3]]
		segments := strings.Split(path, ".")
		imports = append(imports, ParsedImport{
			Module: path,
			Alias:  segments[len(segments)-1],
			Line:   LineNumberForOffset(masked, match[0]),
		})
	}
	return imports
}

func rustImports(masked string) []ParsedImport {
	imports := make([]ParsedImport, 0, 4)
	for _, match := range rustUsePattern.FindAllStringSubmatchIndex(masked, -1) {
		clause := squashWhitespace(masked[match[2]:match[3]])
		line := LineNumberForOffset(masked, match[0])
		imports = append(imports, rustUseClauseImports(clause, line)...)
	}
	return imports
}

func rustUseClauseImports(clause string, line int) []ParsedImport {
	if grouped := rustUseGroupedPattern.FindStringSubmatch(clause); grouped != nil {
		imports := make([]ParsedImport, 0, 2)
		for _, part := range strings.Split(grouped[2], ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			imports = append(imports, rustUseImport(grouped[1]+"::"+part, line))
		}
		return imports
	}
	return []ParsedImport{rustUseImport(clause, line)}
}

func rustUseImport(path string, line int) ParsedImport {
	alias := ""
	if fields := strings.Fields(path); len(fields) == 3 && fields[1] == "as" {
		path = fields[0]
		alias = fields[2]
	}
	if alias == "" {
		segments := strings.Split(path, "::")
		alias = segments[len(segments)-1]
	}
	return ParsedImport{Module: path, Alias: alias, Line: line}
}
