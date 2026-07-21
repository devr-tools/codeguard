package security

import (
	"go/ast"
	"path"
	"strconv"
)

// goFrameworkModel is the small, internal schema for framework-aware Go
// taint models. A model is active only after its import and a typed binding
// are both present, which prevents lookalike local types from changing the
// generic analyzer's behavior.
type goFrameworkModel struct {
	name          string
	importPath    string
	requestType   string
	sourceMethods map[string]bool
	databaseType  string
	sinkMethods   map[string]int
}

var goFrameworkModels = []goFrameworkModel{
	{
		name:        "net/http",
		importPath:  "net/http",
		requestType: "Request",
	},
	{
		name:        "gin",
		importPath:  "github.com/gin-gonic/gin",
		requestType: "Context",
		sourceMethods: map[string]bool{
			"Query": true, "DefaultQuery": true, "Param": true, "PostForm": true,
		},
	},
	{
		name:         "gorm",
		importPath:   "gorm.io/gorm",
		databaseType: "DB",
		sinkMethods:  map[string]int{"Raw": 0, "Exec": 0},
	},
}

type goModelBindings struct {
	imports   map[string]string
	requests  map[string]string
	contexts  map[string]string
	databases map[string]*goFrameworkModel
}

func newGoModelBindings(file *ast.File) goModelBindings {
	bindings := goModelBindings{imports: map[string]string{}, requests: map[string]string{}, contexts: map[string]string{}, databases: map[string]*goFrameworkModel{}}
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		alias := path.Base(importPath)
		if spec.Name != nil {
			alias = spec.Name.Name
		}
		if alias != "." && alias != "_" {
			bindings.imports[alias] = importPath
		}
	}
	return bindings
}

func (b *goModelBindings) bindParam(name string, typ ast.Expr) {
	alias, typeName, ok := importedType(typ)
	if !ok {
		return
	}
	for index := range goFrameworkModels {
		model := &goFrameworkModels[index]
		if b.imports[alias] != model.importPath {
			continue
		}
		if model.requestType == typeName {
			if model.name == "net/http" {
				b.requests[name] = model.name
			} else {
				b.contexts[name] = model.name
			}
		}
		if model.databaseType == typeName {
			b.databases[name] = model
		}
	}
}

func importedType(expr ast.Expr) (alias string, typeName string, ok bool) {
	for {
		switch typed := expr.(type) {
		case *ast.StarExpr:
			expr = typed.X
		case *ast.ParenExpr:
			expr = typed.X
		case *ast.SelectorExpr:
			ident, isIdent := typed.X.(*ast.Ident)
			if !isIdent {
				return "", "", false
			}
			return ident.Name, typed.Sel.Name, true
		default:
			return "", "", false
		}
	}
}

func (b *goModelBindings) sourceModel(receiver string, method string) string {
	if model, ok := b.requests[receiver]; ok {
		return model
	}
	if model, ok := b.contexts[receiver]; ok {
		for _, candidate := range goFrameworkModels {
			if candidate.name == model && candidate.sourceMethods[method] {
				return candidate.name
			}
		}
	}
	return ""
}

func (b *goModelBindings) sinkModel(receiver string, method string) (*goFrameworkModel, int, bool) {
	model, ok := b.databases[receiver]
	if !ok {
		return nil, 0, false
	}
	index, ok := model.sinkMethods[method]
	return model, index, ok
}
