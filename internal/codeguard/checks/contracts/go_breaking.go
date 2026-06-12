package contracts

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type goSymbol struct {
	signature string
	line      int
}

func goBreakingFindings(env support.Context, target core.TargetConfig, changed []core.ChangedFile) []core.Finding {
	if !enabled(env.Config.Checks.ContractRules.GoExportedBreaking) {
		return nil
	}
	findings := make([]core.Finding, 0)
	for _, file := range changed {
		if !strings.HasSuffix(file.Path, ".go") || strings.HasSuffix(file.Path, "_test.go") || file.Status == core.ChangedFileAdded {
			continue
		}
		findings = append(findings, goFileBreakingFindings(env, target, file)...)
	}
	return findings
}

func goFileBreakingFindings(env support.Context, target core.TargetConfig, file core.ChangedFile) []core.Finding {
	baseSymbols, err := goExportedSymbols(readBase(env, target, file.Path))
	if err != nil || len(baseSymbols) == 0 {
		return nil
	}
	headSymbols := map[string]goSymbol{}
	if file.Status != core.ChangedFileDeleted {
		// A head version that fails to parse is reported by other tooling;
		// skip rather than flag every symbol as removed.
		headSymbols, err = goExportedSymbols(readHead(target, file.Path))
		if err != nil {
			return nil
		}
	}
	findings := make([]core.Finding, 0)
	for _, name := range sortedKeys(baseSymbols) {
		baseSymbol := baseSymbols[name]
		headSymbol, ok := headSymbols[name]
		if !ok {
			findings = append(findings, newGoBreakingFinding(env, file.Path, 0,
				fmt.Sprintf("exported %s was removed or renamed against the base ref", name)))
			continue
		}
		if baseSymbol.signature != "" && headSymbol.signature != baseSymbol.signature {
			findings = append(findings, newGoBreakingFinding(env, file.Path, headSymbol.line,
				fmt.Sprintf("exported %s changed signature from %s to %s", name, baseSymbol.signature, headSymbol.signature)))
		}
	}
	return findings
}

func newGoBreakingFinding(env support.Context, path string, line int, message string) core.Finding {
	return env.NewFinding(support.FindingInput{
		RuleID:  "contracts.go-exported-breaking",
		Level:   "fail",
		Path:    path,
		Line:    line,
		Message: message,
	})
}

func goExportedSymbols(src []byte) (map[string]goSymbol, error) {
	if len(src) == 0 {
		return nil, nil
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "contract.go", src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	symbols := map[string]goSymbol{}
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			collectFuncSymbol(fset, d, symbols)
		case *ast.GenDecl:
			collectGenSymbols(fset, d, symbols)
		}
	}
	return symbols, nil
}

func collectFuncSymbol(fset *token.FileSet, decl *ast.FuncDecl, out map[string]goSymbol) {
	if !decl.Name.IsExported() {
		return
	}
	key := "func " + decl.Name.Name
	if decl.Recv != nil && len(decl.Recv.List) > 0 {
		receiver := receiverTypeName(decl.Recv.List[0].Type)
		if !ast.IsExported(receiver) {
			return
		}
		key = fmt.Sprintf("method %s.%s", receiver, decl.Name.Name)
	}
	out[key] = goSymbol{
		signature: funcSignature(fset, decl.Type),
		line:      fset.Position(decl.Pos()).Line,
	}
}

func collectGenSymbols(fset *token.FileSet, decl *ast.GenDecl, out map[string]goSymbol) {
	for _, spec := range decl.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			if decl.Tok == token.TYPE && s.Name.IsExported() {
				out["type "+s.Name.Name] = goSymbol{line: fset.Position(s.Pos()).Line}
			}
		case *ast.ValueSpec:
			if decl.Tok != token.CONST {
				continue
			}
			for _, name := range s.Names {
				if name.IsExported() {
					out["const "+name.Name] = goSymbol{line: fset.Position(name.Pos()).Line}
				}
			}
		}
	}
}

func receiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return receiverTypeName(t.X)
	case *ast.IndexExpr:
		return receiverTypeName(t.X)
	case *ast.IndexListExpr:
		return receiverTypeName(t.X)
	case *ast.Ident:
		return t.Name
	default:
		return ""
	}
}

// funcSignature renders parameter and result types (names excluded, so a
// parameter rename is not treated as a breaking change).
func funcSignature(fset *token.FileSet, fn *ast.FuncType) string {
	signature := "(" + strings.Join(fieldListTypes(fset, fn.Params), ", ") + ")"
	if results := fieldListTypes(fset, fn.Results); len(results) > 0 {
		signature += " (" + strings.Join(results, ", ") + ")"
	}
	if fn.TypeParams != nil {
		signature = "[" + strings.Join(fieldListTypes(fset, fn.TypeParams), ", ") + "]" + signature
	}
	return signature
}

func fieldListTypes(fset *token.FileSet, fields *ast.FieldList) []string {
	if fields == nil {
		return nil
	}
	types := make([]string, 0, len(fields.List))
	for _, field := range fields.List {
		text := exprText(fset, field.Type)
		count := len(field.Names)
		if count == 0 {
			count = 1
		}
		for i := 0; i < count; i++ {
			types = append(types, text)
		}
	}
	return types
}

func exprText(fset *token.FileSet, expr ast.Expr) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, expr); err != nil {
		return ""
	}
	return buf.String()
}
