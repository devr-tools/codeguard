package performance

import (
	"bytes"
	"go/ast"
	"go/printer"
	"go/token"
	"path"
	"strings"
)

var syncIOOperationsByImportPath = map[string]map[string]struct{}{
	"os": {
		"Create":    {},
		"Lstat":     {},
		"Open":      {},
		"OpenFile":  {},
		"ReadDir":   {},
		"ReadFile":  {},
		"Stat":      {},
		"WriteFile": {},
	},
	"io/ioutil": {
		"ReadDir":   {},
		"ReadFile":  {},
		"WriteFile": {},
	},
}

func syncIOAliases(parsed *ast.File) map[string]map[string]struct{} {
	aliases := make(map[string]map[string]struct{})
	for _, imp := range parsed.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		operations, ok := syncIOOperationsByImportPath[importPath]
		if !ok {
			continue
		}
		alias := importLocalName(imp, importPath)
		if alias == "" {
			continue
		}
		aliases[alias] = operations
	}
	return aliases
}

func importAliasesForPath(parsed *ast.File, importPath string) map[string]struct{} {
	aliases := make(map[string]struct{})
	for _, imp := range parsed.Imports {
		if strings.Trim(imp.Path.Value, `"`) != importPath {
			continue
		}
		if alias := importLocalName(imp, importPath); alias != "" {
			aliases[alias] = struct{}{}
		}
	}
	return aliases
}

func importLocalName(imp *ast.ImportSpec, importPath string) string {
	if imp.Name != nil {
		switch imp.Name.Name {
		case "_", ".":
			return ""
		default:
			return imp.Name.Name
		}
	}
	return path.Base(importPath)
}

func normalizedExprString(expr ast.Expr) string {
	var buf bytes.Buffer
	_ = printer.Fprint(&buf, token.NewFileSet(), expr)
	return strings.ReplaceAll(buf.String(), " ", "")
}
