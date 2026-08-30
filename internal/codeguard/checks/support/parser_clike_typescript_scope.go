package support

import (
	"regexp"
	"strings"
)

var (
	typeScriptDeclarationPattern = regexp.MustCompile(`(?m)\b(?:const|let|var)[ \t]+([A-Za-z_$][\w$]*)[ \t]*(?::[ \t]*[^=;\n]+)?[ \t]*(?:=[ \t]*([^;\n]*))?`)
	typeScriptAliasPattern       = regexp.MustCompile(`^[ \t]*([A-Za-z_$][\w$]*)[ \t]*$`)
)

func populateTypeScriptDeclarationMetadata(file *ParsedFile) {
	for _, fn := range file.AllFunctions() {
		if fn.bodyOpen < 0 || fn.bodyEnd <= fn.bodyOpen {
			continue
		}
		body := file.Masked[fn.bodyOpen+1 : fn.bodyEnd]
		for _, match := range typeScriptDeclarationPattern.FindAllStringSubmatchIndex(body, -1) {
			name := body[match[2]:match[3]]
			initializer := ""
			if match[4] >= 0 {
				initializer = strings.TrimSpace(body[match[4]:match[5]])
			}
			offset := fn.bodyOpen + 1 + match[2]
			scopeStart, scopeEnd := clikeLexicalScope(file.Masked, fn.bodyOpen, fn.bodyEnd, offset)
			aliasSource := ""
			if alias := typeScriptAliasPattern.FindStringSubmatch(initializer); len(alias) == 2 {
				aliasSource = alias[1]
			}
			fn.Declarations = append(fn.Declarations, ParsedDeclaration{
				Name: name, Kind: "local", Line: LineNumberForOffset(file.Source, offset),
				ScopeStart: LineNumberForOffset(file.Source, scopeStart), ScopeEnd: LineNumberForOffset(file.Source, scopeEnd),
				Offset: offset, ScopeOffsetStart: scopeStart, ScopeOffsetEnd: scopeEnd,
				AliasSource: aliasSource, Initializer: initializer,
			})
		}
	}
}
