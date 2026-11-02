package support_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// cacheSectionScanFuncs are the per-file scan entry points that take a cache
// section id. The id is argument 1 of a Context hook call (env.ScanTargetFiles(target, id, ...))
// and argument 2 of a package-level helper call (support.ScanGoFiles(env, target, id, ...)).
var cacheSectionScanFuncs = map[string]bool{
	"ScanTargetFiles":                 true,
	"ScanTargetFilesWithDependencies": true,
	"ScanTargetFilesWithDiagnostics":  true,
	"ScanTargetFilesSequential":       true,
	"ScanGoFiles":                     true,
	"ScanPythonFiles":                 true,
	"ScanRustFiles":                   true,
	"ScanCPPFiles":                    true,
}

// cacheSectionFamilies are the id prefixes (the text before the first '-')
// that a cache section id may use. The first group are the config families
// sectionConfigFamily scopes invalidation to; the rest fall back to the
// all-checks fingerprint.
var cacheSectionFamilies = map[string]bool{
	"quality": true, "performance": true, "design": true, "security": true,
	"prompts": true, "ci": true, "contracts": true,
	"reliability": true, "data": true, "observability": true, "custom": true,
}

var cacheSectionsFile = filepath.Join("..", "..", "internal", "codeguard", "checks", "support", "cache_sections.go")

// TestCacheSectionIDsAreUniquePerPass makes a shared per-file cache section id
// impossible to merge. The findings cache keys an entry on (section id, target,
// file), so two passes that share an id and visit the same file replay each
// other's findings; that shipped twice for "design". Every scan call must name
// a constant from checks/support/cache_sections.go (only the scan plumbing may
// forward a parameter of the enclosing function), and each constant must back
// exactly one call site.
func TestCacheSectionIDsAreUniquePerPass(t *testing.T) {
	constants := loadCacheSectionConstants(t)
	assertCacheSectionConstantsWellFormed(t, constants)

	uses := map[string][]string{}
	var violations []string
	root := filepath.Join("..", "..", "internal")
	walkErr := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		collectCacheSectionUses(fset, file, constants, isCacheSectionForwarder(path), uses, &violations)
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk %s: %v", root, walkErr)
	}

	for _, violation := range violations {
		t.Errorf("%s", violation)
	}
	names := make([]string, 0, len(constants))
	for name := range constants {
		names = append(names, name)
	}
	sort.Strings(names)
	total := 0
	for _, name := range names {
		sites := uses[name]
		total += len(sites)
		switch len(sites) {
		case 1:
		case 0:
			t.Errorf("cache section constant %s (%q) is not used by any scan call; remove it", name, constants[name])
		default:
			t.Errorf("cache section constant %s (%q) is shared by %d scan calls (%s); give each pass its own id",
				name, constants[name], len(sites), strings.Join(sites, ", "))
		}
	}
	// Guards against the walk silently matching nothing (a renamed helper or
	// a moved tree) and the test passing vacuously.
	if total < 30 {
		t.Fatalf("found only %d cache section uses; the scan-call detection is probably stale", total)
	}
}

func loadCacheSectionConstants(t *testing.T) map[string]string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, cacheSectionsFile, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", cacheSectionsFile, err)
	}
	constants := map[string]string{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			value := spec.(*ast.ValueSpec)
			for i, name := range value.Names {
				lit, ok := value.Values[i].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					t.Fatalf("cache section constant %s must be a string literal", name.Name)
				}
				unquoted, err := strconv.Unquote(lit.Value)
				if err != nil {
					t.Fatalf("unquote %s: %v", name.Name, err)
				}
				constants[name.Name] = unquoted
			}
		}
	}
	return constants
}

// assertCacheSectionConstantsWellFormed checks that ids are unique, that each
// id keeps a known config-family prefix, and that each constant's name spells
// its id (CacheSectionQualityGo is "quality-go"), so two constants can only
// carry the same id if they also carry the same name, which does not compile.
func assertCacheSectionConstantsWellFormed(t *testing.T, constants map[string]string) {
	t.Helper()
	byValue := map[string]string{}
	for name, value := range constants {
		if other, ok := byValue[value]; ok {
			t.Errorf("cache section constants %s and %s share the id %q", name, other, value)
		}
		byValue[value] = name
		family, _, _ := strings.Cut(value, "-")
		if !cacheSectionFamilies[family] {
			t.Errorf("cache section id %q (%s) has unknown family prefix %q; keep the section's prefix so config invalidation stays scoped", value, name, family)
		}
		if !strings.HasPrefix(name, "CacheSection") {
			t.Errorf("cache section constant %s must be named CacheSection<Id>", name)
			continue
		}
		spelled := strings.ToLower(strings.TrimPrefix(name, "CacheSection"))
		if spelled != strings.ReplaceAll(value, "-", "") {
			t.Errorf("cache section constant %s does not spell its id %q", name, value)
		}
	}
}

// cacheSectionForwarderDirs hold the scan plumbing itself: the helpers and
// Context hooks that receive an id from their caller and pass it on. Only
// there may a scan call forward a parameter instead of naming a constant;
// anywhere else a wrapper taking an id would hide a shared literal from this
// test.
var cacheSectionForwarderDirs = []string{
	"internal/codeguard/checks/support/",
	"internal/codeguard/runner/support/",
	"internal/codeguard/runner/checks/",
}

func isCacheSectionForwarder(path string) bool {
	slashed := filepath.ToSlash(path)
	for _, dir := range cacheSectionForwarderDirs {
		if strings.Contains(slashed, dir) {
			return true
		}
	}
	return false
}

func collectCacheSectionUses(fset *token.FileSet, file *ast.File, constants map[string]string, forwarder bool, uses map[string][]string, violations *[]string) {
	imports := importNames(file)
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		params := funcParamNames(fn)
		record := func(expr ast.Expr, where string) {
			position := fset.Position(expr.Pos())
			site := filepath.ToSlash(position.Filename) + ":" + strconv.Itoa(position.Line)
			if name, ok := cacheSectionConstantRef(expr, constants); ok {
				uses[name] = append(uses[name], site)
				return
			}
			if root := rootIdent(expr); forwarder && root != "" && params[root] {
				return // forwards the caller's id, which is checked at the caller
			}
			*violations = append(*violations, site+": "+where+" must pass a support.CacheSection* constant as its cache section id")
		}
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			switch value := node.(type) {
			case *ast.CallExpr:
				if name, index, ok := cacheSectionScanCall(value, imports); ok && index < len(value.Args) {
					record(value.Args[index], name)
				}
			case *ast.CompositeLit:
				if typeName(value.Type) != "TypeScriptTargetScan" {
					return true
				}
				for _, elt := range value.Elts {
					if kv, ok := elt.(*ast.KeyValueExpr); ok {
						if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "SectionID" {
							record(kv.Value, "TypeScriptTargetScan.SectionID")
						}
					}
				}
			}
			return true
		})
	}
}

// cacheSectionScanCall reports whether call invokes a per-file scan entry
// point and, if so, which argument carries the cache section id.
func cacheSectionScanCall(call *ast.CallExpr, imports map[string]bool) (string, int, bool) {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		if cacheSectionScanFuncs[fun.Name] {
			return fun.Name, 2, true
		}
	case *ast.SelectorExpr:
		if !cacheSectionScanFuncs[fun.Sel.Name] {
			return "", 0, false
		}
		if x, ok := fun.X.(*ast.Ident); ok && imports[x.Name] {
			return fun.Sel.Name, 2, true
		}
		return fun.Sel.Name, 1, true
	}
	return "", 0, false
}

func cacheSectionConstantRef(expr ast.Expr, constants map[string]string) (string, bool) {
	switch value := expr.(type) {
	case *ast.Ident:
		_, ok := constants[value.Name]
		return value.Name, ok
	case *ast.SelectorExpr:
		if _, ok := value.X.(*ast.Ident); !ok {
			return "", false
		}
		_, ok := constants[value.Sel.Name]
		return value.Sel.Name, ok
	}
	return "", false
}

func rootIdent(expr ast.Expr) string {
	for {
		switch value := expr.(type) {
		case *ast.Ident:
			return value.Name
		case *ast.SelectorExpr:
			expr = value.X
		default:
			return ""
		}
	}
}

func typeName(expr ast.Expr) string {
	switch value := expr.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.SelectorExpr:
		return value.Sel.Name
	}
	return ""
}

func importNames(file *ast.File) map[string]bool {
	names := map[string]bool{}
	for _, spec := range file.Imports {
		if spec.Name != nil {
			names[spec.Name.Name] = true
			continue
		}
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		names[path[strings.LastIndex(path, "/")+1:]] = true
	}
	return names
}

// funcParamNames returns the parameter names of fn and of every function
// literal inside it, so an id forwarded through a closure is recognised.
func funcParamNames(fn *ast.FuncDecl) map[string]bool {
	names := map[string]bool{}
	addFields := func(fields *ast.FieldList) {
		if fields == nil {
			return
		}
		for _, field := range fields.List {
			for _, name := range field.Names {
				names[name.Name] = true
			}
		}
	}
	addFields(fn.Type.Params)
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		if lit, ok := node.(*ast.FuncLit); ok {
			addFields(lit.Type.Params)
		}
		return true
	})
	return names
}
