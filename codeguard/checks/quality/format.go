package quality

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"

	"github.com/devr-tools/codeguard/codeguard/core"
)

func runFormatChecks(path string) ([]byte, *ast.File, *token.FileSet, []core.Finding, core.Severity) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, nil, []core.Finding{{
			Path:     filepath.ToSlash(path),
			Message:  err.Error(),
			Severity: core.SeverityError,
		}}, core.SeverityError
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, source, parser.AllErrors)
	if err != nil {
		return nil, nil, nil, []core.Finding{{
			Path:     filepath.ToSlash(path),
			Message:  fmt.Sprintf("parse error: %v", err),
			Severity: core.SeverityError,
		}}, core.SeverityError
	}

	formatted, err := format.Source(source)
	if err != nil {
		return nil, nil, nil, []core.Finding{{
			Path:     filepath.ToSlash(path),
			Message:  fmt.Sprintf("format error: %v", err),
			Severity: core.SeverityError,
		}}, core.SeverityError
	}
	if !bytes.Equal(source, formatted) {
		return nil, nil, nil, []core.Finding{{
			Path:     filepath.ToSlash(path),
			Message:  "file is not gofmt-formatted",
			Severity: core.SeverityError,
		}}, core.SeverityError
	}

	return source, file, fset, nil, core.SeverityInfo
}
