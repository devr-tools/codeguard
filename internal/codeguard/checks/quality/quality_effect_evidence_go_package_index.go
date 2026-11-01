package quality

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"strings"
)

type goPackageVariable struct {
	name string
	typ  ast.Expr
	init ast.Expr
	line int
}

type goPackageInfo struct {
	name          string
	dir           string
	importsByFile map[string]map[string]struct{}
	types         map[string]ast.Expr
	variables     map[string]goPackageVariable
}

type goPackageIndex struct {
	packages   map[string]*goPackageInfo
	fileToInfo map[string]*goPackageInfo
}

func newGoPackageIndex() *goPackageIndex {
	return &goPackageIndex{
		packages:   make(map[string]*goPackageInfo),
		fileToInfo: make(map[string]*goPackageInfo),
	}
}

func goPackageKey(file string, packageName string) string {
	return filepath.Clean(filepath.Dir(file)) + "\x00" + packageName
}

func (index *goPackageIndex) addFile(file string, fset *token.FileSet, parsed *ast.File) {
	if index == nil || parsed == nil || parsed.Name == nil {
		return
	}
	key := goPackageKey(file, parsed.Name.Name)
	pkg := index.packages[key]
	if pkg == nil {
		pkg = &goPackageInfo{
			name:          parsed.Name.Name,
			dir:           filepath.Clean(filepath.Dir(file)),
			importsByFile: make(map[string]map[string]struct{}),
			types:         make(map[string]ast.Expr),
			variables:     make(map[string]goPackageVariable),
		}
		index.packages[key] = pkg
	}
	cleanFile := filepath.Clean(file)
	index.fileToInfo[cleanFile] = pkg
	pkg.importsByFile[cleanFile] = goFileImportNames(parsed)
	for _, declaration := range parsed.Decls {
		if general, ok := declaration.(*ast.GenDecl); ok {
			pkg.addDeclaration(fset, general)
		}
	}
}

// goFileImportNames returns the package names a file's imports bind,
// excluding dot and blank imports.
func goFileImportNames(parsed *ast.File) map[string]struct{} {
	imports := make(map[string]struct{})
	for _, spec := range parsed.Imports {
		name := ""
		if spec.Name != nil {
			name = spec.Name.Name
		} else {
			path := strings.Trim(spec.Path.Value, `"`)
			name = filepath.Base(path)
		}
		if name != "" && name != "." && name != "_" {
			imports[name] = struct{}{}
		}
	}
	return imports
}

// addDeclaration records the type declarations and package-level
// variables/constants of one general declaration.
func (pkg *goPackageInfo) addDeclaration(fset *token.FileSet, general *ast.GenDecl) {
	for _, rawSpec := range general.Specs {
		switch spec := rawSpec.(type) {
		case *ast.TypeSpec:
			pkg.types[spec.Name.Name] = spec.Type
		case *ast.ValueSpec:
			if general.Tok != token.VAR && general.Tok != token.CONST {
				continue
			}
			for position, name := range spec.Names {
				line := 0
				if fset != nil {
					line = fset.Position(name.Pos()).Line
				}
				pkg.variables[name.Name] = goPackageVariable{name: name.Name, typ: spec.Type, init: goValueAt(spec.Values, position), line: line}
			}
		}
	}
}

// goValueAt returns the value assigned to the name at position in a
// declaration or assignment: the matching value, or the single value of a
// multi-value expression.
func goValueAt(values []ast.Expr, position int) ast.Expr {
	if position < len(values) {
		return values[position]
	}
	if len(values) == 1 {
		return values[0]
	}
	return nil
}

func (index *goPackageIndex) packageFor(file string, packageName string) *goPackageInfo {
	if index == nil {
		return nil
	}
	if pkg := index.fileToInfo[filepath.Clean(file)]; pkg != nil {
		return pkg
	}
	return index.packages[goPackageKey(file, packageName)]
}
