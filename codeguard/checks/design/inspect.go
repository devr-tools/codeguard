package design

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"

	"github.com/devr-tools/codeguard/codeguard/core"
)

func inspectFile(targetRoot string, path string, rules designRules) ([]core.Finding, core.Severity) {
	source, err := os.ReadFile(path)
	if err != nil {
		return []core.Finding{{
			Path:     filepath.ToSlash(path),
			Message:  err.Error(),
			Severity: core.SeverityError,
		}}, core.SeverityError
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, source, parser.ParseComments)
	if err != nil {
		return []core.Finding{{
			Path:     filepath.ToSlash(path),
			Message:  err.Error(),
			Severity: core.SeverityError,
		}}, core.SeverityError
	}

	relativePath, err := filepath.Rel(targetRoot, path)
	if err != nil {
		relativePath = path
	}
	relativePath = filepath.ToSlash(relativePath)
	var findings []core.Finding
	var severity core.Severity = core.SeverityInfo

	architecture := layerFindings(relativePath, file, rules)
	if len(architecture) > 0 {
		findings = append(findings, architecture...)
		severity = core.SeverityError
	}

	principles := principleFindings(relativePath, file, rules)
	if len(principles) > 0 {
		findings = append(findings, principles...)
		if severity != core.SeverityError {
			severity = core.SeverityWarn
		}
	}

	if len(findings) == 0 {
		return nil, core.SeverityInfo
	}
	return findings, severity
}

func importPaths(file *ast.File) []string {
	paths := make([]string, 0, len(file.Imports))
	for _, imp := range file.Imports {
		if imp == nil || imp.Path == nil {
			continue
		}
		paths = append(paths, trimImportPath(imp.Path.Value))
	}
	return paths
}
