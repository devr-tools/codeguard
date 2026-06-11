package quality

import (
	"fmt"
	"go/ast"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/codeguard/core"
)

func dependencyDirectionFindings(path string, file *ast.File) []core.Finding {
	cleanPath := filepath.ToSlash(path)
	if strings.HasPrefix(cleanPath, "cmd/") || strings.Contains(cleanPath, "/cmd/") || strings.HasPrefix(cleanPath, "tests/") {
		return nil
	}

	var findings []core.Finding
	for _, imp := range file.Imports {
		importPath := strings.Trim(imp.Path.Value, "\"")
		if strings.Contains(importPath, "/cmd/") || strings.HasSuffix(importPath, "/cmd") {
			findings = append(findings, core.Finding{
				Path:     cleanPath,
				Message:  fmt.Sprintf("reusable code imports command package %s", importPath),
				Severity: core.SeverityWarn,
			})
		}
		if strings.Contains(importPath, "/internal/cli") {
			findings = append(findings, core.Finding{
				Path:     cleanPath,
				Message:  fmt.Sprintf("reusable code imports CLI package %s", importPath),
				Severity: core.SeverityWarn,
			})
		}
	}
	return findings
}
