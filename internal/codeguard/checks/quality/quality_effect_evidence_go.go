package quality

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
)

const goAliasHopLimit = 32

type goScopeID int

type goSymbolKind string

const (
	goSymbolImport    goSymbolKind = "import"
	goSymbolGlobal    goSymbolKind = "global"
	goSymbolReceiver  goSymbolKind = "receiver"
	goSymbolParameter goSymbolKind = "parameter"
	goSymbolLocal     goSymbolKind = "local"
	goSymbolRange     goSymbolKind = "range"
	goSymbolResult    goSymbolKind = "result"
)

type goReferenceKind uint8

const (
	goShapeUnknown goReferenceKind = iota
	goShapeValue
	goShapePointer
	goShapeMap
	goShapeSlice
	goShapeArray
	goShapeInterface
	goShapePackage
)

type goReferenceShape struct {
	Kind   goReferenceKind
	Elem   *goReferenceShape
	Fields map[string]goReferenceShape
	Name   string
}

func (shape goReferenceShape) referenceBacked() bool {
	switch shape.Kind {
	case goShapePointer, goShapeMap, goShapeSlice, goShapeInterface:
		return true
	default:
		return false
	}
}

type goSymbol struct {
	ID       int
	Name     string
	Kind     goSymbolKind
	Scope    goScopeID
	Shape    goReferenceShape
	Origin   string
	AliasOf  int
	DeclLine int

	Target    string
	ContentOf int
}

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
	pkg.importsByFile[cleanFile] = imports
	for _, declaration := range parsed.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, rawSpec := range general.Specs {
			switch spec := rawSpec.(type) {
			case *ast.TypeSpec:
				pkg.types[spec.Name.Name] = spec.Type
			case *ast.ValueSpec:
				if general.Tok != token.VAR && general.Tok != token.CONST {
					continue
				}
				for position, name := range spec.Names {
					var initial ast.Expr
					if position < len(spec.Values) {
						initial = spec.Values[position]
					} else if len(spec.Values) == 1 {
						initial = spec.Values[0]
					}
					line := 0
					if fset != nil {
						line = fset.Position(name.Pos()).Line
					}
					pkg.variables[name.Name] = goPackageVariable{name: name.Name, typ: spec.Type, init: initial, line: line}
				}
			}
		}
	}
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

type goScope struct {
	id      goScopeID
	parent  *goScope
	symbols map[string]*goSymbol
}

func (scope *goScope) lookup(name string) *goSymbol {
	for current := scope; current != nil; current = current.parent {
		if symbol := current.symbols[name]; symbol != nil {
			return symbol
		}
	}
	return nil
}

func snapshotGoScope(scope *goScope) *goScope {
	if scope == nil {
		return nil
	}
	snapshot := &goScope{id: scope.id, parent: snapshotGoScope(scope.parent), symbols: make(map[string]*goSymbol, len(scope.symbols))}
	for name, symbol := range scope.symbols {
		snapshot.symbols[name] = symbol
	}
	return snapshot
}

type goExpressionPath struct {
	symbol           *goSymbol
	shape            goReferenceShape
	crossedReference bool
	unresolved       string
	packageName      bool
	bindingKey       string
}

type goClosureDefinition struct {
	literal *ast.FuncLit
	scope   *goScope
}

type goMutationResolver struct {
	fn        precisionFunction
	pkg       *goPackageInfo
	fset      *token.FileSet
	nextID    int
	nextScope goScopeID
	symbols   map[int]*goSymbol
	escapedAt map[int]int
	analysis  mutationAnalysis
	seen      map[string]struct{}
	unseen    map[string]struct{}
	order     int
	closures  map[int]goClosureDefinition
	active    map[int]bool
	fields    map[string]*goSymbol
}

func goFunctionMutationEvidence(fn precisionFunction) mutationAnalysis {
	if fn.GoDecl == nil || fn.GoDecl.Body == nil {
		return mutationAnalysis{}
	}
	resolver := &goMutationResolver{
		fn:        fn,
		pkg:       fn.GoPackage,
		fset:      fn.GoFSet,
		symbols:   make(map[int]*goSymbol),
		escapedAt: make(map[int]int),
		seen:      make(map[string]struct{}),
		unseen:    make(map[string]struct{}),
		closures:  make(map[int]goClosureDefinition),
		active:    make(map[int]bool),
		fields:    make(map[string]*goSymbol),
	}
	packageScope := resolver.newScope(nil)
	resolver.declarePackageSymbols(packageScope)
	functionScope := resolver.newScope(packageScope)
	resolver.declareFunctionFields(functionScope, fn.GoDecl)
	resolver.walkStatements(functionScope, fn.GoDecl.Body.List)
	return resolver.analysis
}

func (resolver *goMutationResolver) newScope(parent *goScope) *goScope {
	resolver.nextScope++
	return &goScope{id: resolver.nextScope, parent: parent, symbols: make(map[string]*goSymbol)}
}

func (resolver *goMutationResolver) declare(scope *goScope, name string, kind goSymbolKind, shape goReferenceShape, origin string, target string, line int) *goSymbol {
	if name == "" || name == "_" {
		return nil
	}
	resolver.nextID++
	symbol := &goSymbol{ID: resolver.nextID, Name: name, Kind: kind, Scope: scope.id, Shape: shape, Origin: origin, Target: target, DeclLine: line}
	scope.symbols[name] = symbol
	resolver.symbols[symbol.ID] = symbol
	return symbol
}

func (resolver *goMutationResolver) declarePackageSymbols(scope *goScope) {
	if resolver.pkg != nil {
		for name, variable := range resolver.pkg.variables {
			shape := resolver.typeShape(variable.typ, nil)
			if variable.typ == nil {
				shape = resolver.expressionShape(scope, variable.init)
			}
			resolver.declare(scope, name, goSymbolGlobal, shape, originShared, targetGlobal, variable.line)
		}
		for name := range resolver.pkg.importsByFile[filepath.Clean(resolver.fn.GoFile)] {
			resolver.declare(scope, name, goSymbolImport, goReferenceShape{Kind: goShapePackage, Name: name}, "", "", 0)
		}
		if resolver.fn.GoFile == "" && len(resolver.pkg.importsByFile) == 1 {
			for _, imports := range resolver.pkg.importsByFile {
				for name := range imports {
					resolver.declare(scope, name, goSymbolImport, goReferenceShape{Kind: goShapePackage, Name: name}, "", "", 0)
				}
			}
		}
		return
	}
	for name := range resolver.fn.ProvenGlobals {
		resolver.declare(scope, name, goSymbolGlobal, goReferenceShape{Kind: goShapeUnknown}, originShared, targetGlobal, resolver.fn.StartLine)
	}
}

func (resolver *goMutationResolver) declareFunctionFields(scope *goScope, declaration *ast.FuncDecl) {
	if declaration.Recv != nil {
		for _, field := range declaration.Recv.List {
			shape := resolver.typeShape(field.Type, nil)
			for _, name := range field.Names {
				resolver.declare(scope, name.Name, goSymbolReceiver, shape, originCaller, targetReceiver, resolver.line(name.Pos()))
			}
		}
	}
	resolver.declareFieldList(scope, declaration.Type.Params, goSymbolParameter, originCaller, targetArgument)
	resolver.declareFieldList(scope, declaration.Type.Results, goSymbolResult, originLocal, targetLocal)
}

func (resolver *goMutationResolver) declareFieldList(scope *goScope, fields *ast.FieldList, kind goSymbolKind, origin string, target string) {
	if fields == nil {
		return
	}
	for _, field := range fields.List {
		shape := resolver.typeShape(field.Type, nil)
		for _, name := range field.Names {
			resolver.declare(scope, name.Name, kind, shape, origin, target, resolver.line(name.Pos()))
		}
	}
}

func (resolver *goMutationResolver) line(position token.Pos) int {
	if resolver.fset == nil {
		return resolver.fn.StartLine
	}
	return resolver.fset.Position(position).Line
}

func (resolver *goMutationResolver) typeShape(expression ast.Expr, visiting map[string]bool) goReferenceShape {
	if expression == nil {
		return goReferenceShape{Kind: goShapeUnknown}
	}
	switch value := expression.(type) {
	case *ast.ParenExpr:
		return resolver.typeShape(value.X, visiting)
	case *ast.StarExpr:
		element := resolver.typeShape(value.X, visiting)
		return goReferenceShape{Kind: goShapePointer, Elem: &element}
	case *ast.MapType:
		element := resolver.typeShape(value.Value, visiting)
		return goReferenceShape{Kind: goShapeMap, Elem: &element}
	case *ast.ArrayType:
		element := resolver.typeShape(value.Elt, visiting)
		kind := goShapeArray
		if value.Len == nil {
			kind = goShapeSlice
		}
		return goReferenceShape{Kind: kind, Elem: &element}
	case *ast.Ellipsis:
		element := resolver.typeShape(value.Elt, visiting)
		return goReferenceShape{Kind: goShapeSlice, Elem: &element}
	case *ast.IndexExpr:
		return resolver.typeShape(value.X, visiting)
	case *ast.IndexListExpr:
		return resolver.typeShape(value.X, visiting)
	case *ast.InterfaceType:
		return goReferenceShape{Kind: goShapeInterface}
	case *ast.StructType:
		shape := goReferenceShape{Kind: goShapeValue, Fields: make(map[string]goReferenceShape)}
		if value.Fields != nil {
			for _, field := range value.Fields.List {
				fieldShape := resolver.typeShape(field.Type, visiting)
				for _, name := range field.Names {
					shape.Fields[name.Name] = fieldShape
				}
			}
		}
		return shape
	case *ast.Ident:
		if value.Name == "any" || value.Name == "error" {
			return goReferenceShape{Kind: goShapeInterface, Name: value.Name}
		}
		if resolver.pkg != nil {
			if declaration := resolver.pkg.types[value.Name]; declaration != nil {
				if visiting == nil {
					visiting = make(map[string]bool)
				}
				if visiting[value.Name] {
					return goReferenceShape{Kind: goShapeValue, Name: value.Name}
				}
				visiting[value.Name] = true
				shape := resolver.typeShape(declaration, visiting)
				delete(visiting, value.Name)
				shape.Name = value.Name
				return shape
			}
		}
		return goReferenceShape{Kind: goShapeValue, Name: value.Name}
	case *ast.ChanType, *ast.FuncType:
		return goReferenceShape{Kind: goShapeValue}
	default:
		return goReferenceShape{Kind: goShapeUnknown}
	}
}

func (resolver *goMutationResolver) expressionShape(scope *goScope, expression ast.Expr) goReferenceShape {
	if expression == nil {
		return goReferenceShape{Kind: goShapeUnknown}
	}
	switch value := expression.(type) {
	case *ast.CompositeLit:
		return resolver.typeShape(value.Type, nil)
	case *ast.UnaryExpr:
		if value.Op == token.AND {
			element := resolver.expressionShape(scope, value.X)
			return goReferenceShape{Kind: goShapePointer, Elem: &element}
		}
		return resolver.expressionShape(scope, value.X)
	case *ast.Ident, *ast.SelectorExpr, *ast.IndexExpr, *ast.SliceExpr, *ast.ParenExpr, *ast.StarExpr, *ast.TypeAssertExpr:
		return resolver.resolveExpression(scope, expression).shape
	case *ast.CallExpr:
		if identifier, ok := value.Fun.(*ast.Ident); ok {
			switch identifier.Name {
			case "append":
				if len(value.Args) > 0 {
					return resolver.expressionShape(scope, value.Args[0])
				}
			case "make":
				if len(value.Args) > 0 {
					return resolver.typeShape(value.Args[0], nil)
				}
			case "new":
				if len(value.Args) > 0 {
					element := resolver.typeShape(value.Args[0], nil)
					return goReferenceShape{Kind: goShapePointer, Elem: &element}
				}
			}
		}
		if shape, ok := resolver.conversionShape(scope, value); ok {
			return shape
		}
		return goReferenceShape{Kind: goShapeUnknown}
	case *ast.FuncLit:
		return goReferenceShape{Kind: goShapeValue}
	default:
		return goReferenceShape{Kind: goShapeValue}
	}
}

func (resolver *goMutationResolver) conversionShape(scope *goScope, call *ast.CallExpr) (goReferenceShape, bool) {
	if call == nil || len(call.Args) != 1 {
		return goReferenceShape{}, false
	}
	if !resolver.isTypeExpression(scope, call.Fun) {
		return goReferenceShape{}, false
	}
	return resolver.typeShape(call.Fun, nil), true
}

func (resolver *goMutationResolver) isTypeExpression(scope *goScope, expression ast.Expr) bool {
	switch value := expression.(type) {
	case *ast.ArrayType, *ast.MapType, *ast.InterfaceType, *ast.StructType:
		return true
	case *ast.ParenExpr:
		return resolver.isTypeExpression(scope, value.X)
	case *ast.StarExpr:
		return resolver.isTypeExpression(scope, value.X)
	case *ast.IndexExpr:
		return resolver.isTypeExpression(scope, value.X)
	case *ast.IndexListExpr:
		return resolver.isTypeExpression(scope, value.X)
	case *ast.Ident:
		return (scope == nil || scope.lookup(value.Name) == nil) && resolver.pkg != nil && resolver.pkg.types[value.Name] != nil
	default:
		return false
	}
}

func (resolver *goMutationResolver) freshConversionOperand(scope *goScope, expression ast.Expr) bool {
	switch value := expression.(type) {
	case *ast.ParenExpr:
		return resolver.freshConversionOperand(scope, value.X)
	case *ast.Ident:
		return value.Name == "nil"
	case *ast.BasicLit, *ast.CompositeLit:
		return true
	case *ast.UnaryExpr:
		return value.Op == token.AND && resolver.freshConversionOperand(scope, value.X)
	case *ast.CallExpr:
		if identifier, ok := value.Fun.(*ast.Ident); ok && (identifier.Name == "make" || identifier.Name == "new") {
			return true
		}
		shape, conversion := resolver.conversionShape(scope, value)
		return conversion && resolver.conversionAllocatesFreshStorage(scope, value, shape)
	default:
		return false
	}
}

func (resolver *goMutationResolver) conversionAllocatesFreshStorage(scope *goScope, call *ast.CallExpr, target goReferenceShape) bool {
	if call == nil || len(call.Args) != 1 {
		return false
	}
	if resolver.freshConversionOperand(scope, call.Args[0]) {
		return true
	}
	if target.Kind != goShapeSlice || target.Elem == nil {
		return false
	}
	source := resolver.expressionShape(scope, call.Args[0])
	if resolver.underlyingScalar(source.Name) != "string" {
		return false
	}
	switch resolver.underlyingScalar(target.Elem.Name) {
	case "uint8", "int32":
		return true
	default:
		return false
	}
}

func (resolver *goMutationResolver) underlyingScalar(name string) string {
	seen := make(map[string]bool)
	for name != "" && !seen[name] {
		seen[name] = true
		switch name {
		case "byte", "uint8":
			return "uint8"
		case "rune", "int32":
			return "int32"
		case "string":
			return "string"
		}
		if resolver.pkg == nil {
			return ""
		}
		declaration, ok := resolver.pkg.types[name].(*ast.Ident)
		if !ok {
			return ""
		}
		name = declaration.Name
	}
	return ""
}

func (resolver *goMutationResolver) resolveExpression(scope *goScope, expression ast.Expr) goExpressionPath {
	switch value := expression.(type) {
	case *ast.Ident:
		symbol := scope.lookup(value.Name)
		if symbol == nil {
			return goExpressionPath{shape: goReferenceShape{Kind: goShapeUnknown}, unresolved: value.Name}
		}
		return goExpressionPath{symbol: symbol, shape: symbol.Shape, packageName: symbol.Kind == goSymbolImport, bindingKey: strconv.Itoa(symbol.ID)}
	case *ast.ParenExpr:
		return resolver.resolveExpression(scope, value.X)
	case *ast.StarExpr:
		path := resolver.resolveExpression(scope, value.X)
		if path.shape.Kind == goShapePointer && path.shape.Elem != nil {
			path.shape = *path.shape.Elem
			path.crossedReference = true
		} else if path.shape.Kind == goShapeUnknown {
			path.unresolved = firstNonEmptyString(path.unresolved, resolver.expressionRootName(value.X))
		}
		return path
	case *ast.SelectorExpr:
		path := resolver.resolveExpression(scope, value.X)
		if path.packageName {
			return path
		}
		if path.bindingKey != "" {
			path.bindingKey += "." + value.Sel.Name
			if binding := resolver.fields[path.bindingKey]; binding != nil {
				return goExpressionPath{symbol: binding, shape: binding.Shape, bindingKey: path.bindingKey}
			}
		}
		if path.shape.Kind == goShapePointer {
			path.crossedReference = true
			if path.shape.Elem != nil {
				path.shape = *path.shape.Elem
			}
		}
		if field, ok := path.shape.Fields[value.Sel.Name]; ok {
			path.shape = field
		} else if path.shape.Kind != goShapeInterface {
			path.shape = goReferenceShape{Kind: goShapeUnknown}
		}
		return path
	case *ast.IndexExpr:
		path := resolver.resolveExpression(scope, value.X)
		if path.shape.Kind == goShapeMap || path.shape.Kind == goShapeSlice {
			path.crossedReference = true
		}
		if path.shape.Elem != nil {
			path.shape = *path.shape.Elem
		} else {
			path.shape = goReferenceShape{Kind: goShapeUnknown}
		}
		return path
	case *ast.SliceExpr:
		path := resolver.resolveExpression(scope, value.X)
		// A slice expression retains the backing storage and ownership of its
		// source. Keep the root symbol so append(input[:0], ...) cannot be
		// mistaken for a fresh local copy.
		if path.shape.Kind != goShapeSlice {
			path.shape = resolver.expressionShape(scope, value.X)
		}
		return path
	case *ast.CallExpr:
		if identifier, ok := value.Fun.(*ast.Ident); ok && identifier.Name == "append" && len(value.Args) > 0 {
			path := resolver.resolveExpression(scope, value.Args[0])
			path.shape = resolver.expressionShape(scope, value)
			return path
		}
		if shape, ok := resolver.conversionShape(scope, value); ok {
			if resolver.conversionAllocatesFreshStorage(scope, value, shape) {
				return goExpressionPath{shape: shape}
			}
			path := resolver.resolveExpression(scope, value.Args[0])
			path.shape = shape
			return path
		}
		return goExpressionPath{shape: resolver.expressionShape(scope, expression), unresolved: resolver.expressionRootName(expression)}
	case *ast.TypeAssertExpr:
		path := resolver.resolveExpression(scope, value.X)
		if value.Type != nil {
			asserted := resolver.typeShape(value.Type, nil)
			path.shape = asserted
			if asserted.referenceBacked() {
				path.crossedReference = true
			}
		}
		return path
	default:
		return goExpressionPath{shape: resolver.expressionShape(scope, expression), unresolved: resolver.expressionRootName(expression)}
	}
}

func (resolver *goMutationResolver) expressionRootName(expression ast.Expr) string {
	for {
		switch value := expression.(type) {
		case *ast.Ident:
			return value.Name
		case *ast.SelectorExpr:
			expression = value.X
		case *ast.IndexExpr:
			expression = value.X
		case *ast.ParenExpr:
			expression = value.X
		case *ast.StarExpr:
			expression = value.X
		case *ast.TypeAssertExpr:
			expression = value.X
		default:
			return ""
		}
	}
}

func (resolver *goMutationResolver) resolveAlias(symbol *goSymbol) *goSymbol {
	seen := make(map[int]bool)
	for hops := 0; symbol != nil && symbol.AliasOf != 0 && hops < goAliasHopLimit; hops++ {
		if seen[symbol.ID] {
			return nil
		}
		seen[symbol.ID] = true
		symbol = resolver.symbols[symbol.AliasOf]
	}
	return symbol
}

func (resolver *goMutationResolver) resolveContentOwner(symbol *goSymbol) *goSymbol {
	seen := make(map[int]bool)
	for hops := 0; symbol != nil && hops < goAliasHopLimit; hops++ {
		if seen[symbol.ID] {
			return nil
		}
		seen[symbol.ID] = true
		if symbol.AliasOf != 0 {
			symbol = resolver.symbols[symbol.AliasOf]
			continue
		}
		if symbol.ContentOf != 0 {
			symbol = resolver.symbols[symbol.ContentOf]
			continue
		}
		return symbol
	}
	return nil
}

func (resolver *goMutationResolver) ownership(path goExpressionPath, call bool) (string, string, bool) {
	if path.symbol == nil || path.packageName {
		return "", "", false
	}
	symbol := resolver.resolveAlias(path.symbol)
	if symbol == nil {
		return "", "", false
	}
	if path.crossedReference && path.symbol.ContentOf != 0 {
		if owner := resolver.resolveContentOwner(path.symbol); owner != nil {
			symbol = owner
		}
	}
	if symbol.Kind == goSymbolGlobal {
		return targetGlobal, originShared, true
	}
	if symbol.Origin == originUnknown {
		return "", "", false
	}
	if symbol.Origin == originLocal {
		root := resolver.resolveAlias(path.symbol)
		if root != nil && resolver.escapedAt[root.ID] > 0 && resolver.order > resolver.escapedAt[root.ID] {
			return targetEscaped, originShared, true
		}
		return targetLocal, originLocal, false
	}
	referenceMutation := path.crossedReference
	if call && path.shape.referenceBacked() {
		referenceMutation = true
	}
	if !referenceMutation {
		return targetLocal, originLocal, false
	}
	return symbol.Target, symbol.Origin, symbol.Target != ""
}

func (resolver *goMutationResolver) addMutation(target string, effect string, origin string, line int, detail string) {
	key := target + "|" + effect + "|" + origin + "|" + detail
	if _, exists := resolver.seen[key]; exists {
		return
	}
	resolver.seen[key] = struct{}{}
	resolver.analysis.Mutations = append(resolver.analysis.Mutations, mutationEvidence{Target: target, Effect: effect, Origin: origin, Line: line, Detail: detail})
}

func (resolver *goMutationResolver) addUnresolved(line int, operation string, symbol string, reason string) {
	if symbol == "" {
		return
	}
	key := operation + "|" + symbol + "|" + strconv.Itoa(line)
	if _, exists := resolver.unseen[key]; exists {
		return
	}
	resolver.unseen[key] = struct{}{}
	resolver.analysis.Unresolved = append(resolver.analysis.Unresolved, unresolvedMutationEvidence{
		Language: "go", Line: line, Operation: operation, Symbol: symbol, Reason: reason,
	})
}

func (resolver *goMutationResolver) recordAssignmentMutation(scope *goScope, expression ast.Expr, line int, detail string) {
	path := resolver.resolveExpression(scope, expression)
	if path.packageName {
		return
	}
	target, origin, reportable := resolver.ownership(path, false)
	if reportable {
		resolver.addMutation(target, "shared_state", origin, line, detail)
		return
	}
	if path.symbol == nil || path.symbol.Origin == originUnknown || (path.symbol.Origin == originCaller && path.shape.Kind == goShapeUnknown) {
		resolver.addUnresolved(line, "assignment", firstNonEmptyString(path.unresolved, resolver.expressionRootName(expression)), "symbol ownership or reference shape could not be resolved")
	}
}

func (resolver *goMutationResolver) walkStatements(scope *goScope, statements []ast.Stmt) {
	for _, statement := range statements {
		resolver.walkStatement(scope, statement)
	}
}

func cloneGoEscapeState(state map[int]int) map[int]int {
	clone := make(map[int]int, len(state))
	for symbolID, escapedAt := range state {
		clone[symbolID] = escapedAt
	}
	return clone
}

func intersectGoEscapeStates(first map[int]int, second map[int]int) map[int]int {
	intersection := make(map[int]int)
	for symbolID, firstEscape := range first {
		secondEscape, exists := second[symbolID]
		if !exists {
			continue
		}
		if secondEscape > firstEscape {
			firstEscape = secondEscape
		}
		intersection[symbolID] = firstEscape
	}
	return intersection
}

func cloneGoFieldBindings(bindings map[string]*goSymbol) map[string]*goSymbol {
	clone := make(map[string]*goSymbol, len(bindings))
	for key, symbol := range bindings {
		clone[key] = symbol
	}
	return clone
}

func goReferenceShapesEqual(first goReferenceShape, second goReferenceShape) bool {
	if first.Kind != second.Kind || first.Name != second.Name || (first.Elem == nil) != (second.Elem == nil) || len(first.Fields) != len(second.Fields) {
		return false
	}
	if first.Elem != nil && !goReferenceShapesEqual(*first.Elem, *second.Elem) {
		return false
	}
	for name, firstField := range first.Fields {
		secondField, exists := second.Fields[name]
		if !exists || !goReferenceShapesEqual(firstField, secondField) {
			return false
		}
	}
	return true
}

func (resolver *goMutationResolver) goFieldBindingsEquivalent(first *goSymbol, second *goSymbol) bool {
	if first == nil || second == nil || !goReferenceShapesEqual(first.Shape, second.Shape) {
		return false
	}
	firstOwner := resolver.resolveContentOwner(first)
	secondOwner := resolver.resolveContentOwner(second)
	if firstOwner == nil || secondOwner == nil {
		return false
	}
	firstGlobal := firstOwner.Kind == goSymbolGlobal
	secondGlobal := secondOwner.Kind == goSymbolGlobal
	return firstGlobal == secondGlobal && firstOwner.Origin == secondOwner.Origin && firstOwner.Target == secondOwner.Target
}

func (resolver *goMutationResolver) intersectGoFieldBindings(first map[string]*goSymbol, second map[string]*goSymbol) map[string]*goSymbol {
	intersection := make(map[string]*goSymbol)
	for key, firstSymbol := range first {
		if resolver.goFieldBindingsEquivalent(firstSymbol, second[key]) {
			intersection[key] = firstSymbol
		}
	}
	return intersection
}

type goBranchState struct {
	escapes map[int]int
	fields  map[string]*goSymbol
}

type goSymbolBindingState struct {
	shape     goReferenceShape
	origin    string
	target    string
	aliasOf   int
	contentOf int
}

func snapshotGoSymbolBinding(symbol *goSymbol) goSymbolBindingState {
	return goSymbolBindingState{
		shape: symbol.Shape, origin: symbol.Origin, target: symbol.Target,
		aliasOf: symbol.AliasOf, contentOf: symbol.ContentOf,
	}
}

func restoreGoSymbolBinding(symbol *goSymbol, state goSymbolBindingState) {
	symbol.Shape = state.shape
	symbol.Origin = state.origin
	symbol.Target = state.target
	symbol.AliasOf = state.aliasOf
	symbol.ContentOf = state.contentOf
}

func (resolver *goMutationResolver) branchState() goBranchState {
	return goBranchState{escapes: cloneGoEscapeState(resolver.escapedAt), fields: cloneGoFieldBindings(resolver.fields)}
}

func (resolver *goMutationResolver) restoreBranchState(state goBranchState) {
	resolver.escapedAt = cloneGoEscapeState(state.escapes)
	resolver.fields = cloneGoFieldBindings(state.fields)
}

func (resolver *goMutationResolver) intersectGoBranchStates(first goBranchState, second goBranchState) goBranchState {
	return goBranchState{
		escapes: intersectGoEscapeStates(first.escapes, second.escapes),
		fields:  resolver.intersectGoFieldBindings(first.fields, second.fields),
	}
}

func (resolver *goMutationResolver) walkExclusiveBranches(scope *goScope, statements []ast.Stmt, exhaustive bool) {
	before := resolver.branchState()
	states := make([]goBranchState, 0, len(statements)+1)
	for _, statement := range statements {
		resolver.restoreBranchState(before)
		resolver.walkStatement(scope, statement)
		states = append(states, resolver.branchState())
	}
	if !exhaustive || len(states) == 0 {
		states = append(states, before)
	}
	merged := states[0]
	for _, state := range states[1:] {
		merged = resolver.intersectGoBranchStates(merged, state)
	}
	resolver.restoreBranchState(merged)
}

func goCaseFallsThrough(statement ast.Stmt) bool {
	clause, ok := statement.(*ast.CaseClause)
	if !ok || len(clause.Body) == 0 {
		return false
	}
	branch, ok := clause.Body[len(clause.Body)-1].(*ast.BranchStmt)
	return ok && branch.Tok == token.FALLTHROUGH
}

func (resolver *goMutationResolver) walkSwitchBranches(scope *goScope, statements []ast.Stmt, exhaustive bool) {
	before := resolver.branchState()
	states := make([]goBranchState, 0, len(statements)+1)
	for entry := range statements {
		resolver.restoreBranchState(before)
		resolver.walkStatement(scope, statements[entry])
		for current := entry; goCaseFallsThrough(statements[current]) && current+1 < len(statements); current++ {
			next, ok := statements[current+1].(*ast.CaseClause)
			if !ok {
				break
			}
			resolver.walkStatements(resolver.newScope(scope), next.Body)
		}
		states = append(states, resolver.branchState())
	}
	if !exhaustive || len(states) == 0 {
		states = append(states, before)
	}
	merged := states[0]
	for _, state := range states[1:] {
		merged = resolver.intersectGoBranchStates(merged, state)
	}
	resolver.restoreBranchState(merged)
}

func goBranchesHaveDefault(statements []ast.Stmt) bool {
	for _, statement := range statements {
		switch clause := statement.(type) {
		case *ast.CaseClause:
			if len(clause.List) == 0 {
				return true
			}
		case *ast.CommClause:
			if clause.Comm == nil {
				return true
			}
		}
	}
	return false
}

func (resolver *goMutationResolver) walkStatement(scope *goScope, statement ast.Stmt) {
	resolver.order++
	switch value := statement.(type) {
	case *ast.BlockStmt:
		resolver.walkStatements(resolver.newScope(scope), value.List)
	case *ast.DeclStmt:
		resolver.handleDeclaration(scope, value.Decl)
	case *ast.AssignStmt:
		for _, expression := range value.Rhs {
			resolver.walkExpressionCalls(scope, expression)
		}
		line := resolver.line(value.Pos())
		if value.Tok != token.DEFINE {
			for _, expression := range value.Lhs {
				switch expression.(type) {
				case *ast.SelectorExpr, *ast.IndexExpr, *ast.StarExpr:
					resolver.recordAssignmentMutation(scope, expression, line, resolver.expressionRootName(expression))
				case *ast.Ident:
					if value.Tok != token.ASSIGN {
						path := resolver.resolveExpression(scope, expression)
						if path.symbol != nil && path.symbol.Kind == goSymbolGlobal {
							resolver.recordAssignmentMutation(scope, expression, line, path.symbol.Name)
						}
					}
				}
			}
		}
		resolver.recordEscapes(scope, value)
		resolver.bindAssignment(scope, value)
	case *ast.IncDecStmt:
		resolver.recordAssignmentMutation(scope, value.X, resolver.line(value.Pos()), resolver.expressionRootName(value.X))
	case *ast.ExprStmt:
		resolver.walkExpressionCalls(scope, value.X)
	case *ast.ReturnStmt:
		for _, expression := range value.Results {
			resolver.walkExpressionCalls(scope, expression)
		}
	case *ast.IfStmt:
		control := resolver.newScope(scope)
		if value.Init != nil {
			resolver.walkStatement(control, value.Init)
		}
		resolver.walkExpressionCalls(control, value.Cond)
		beforeBranches := resolver.branchState()
		resolver.restoreBranchState(beforeBranches)
		resolver.walkStatements(resolver.newScope(control), value.Body.List)
		thenState := resolver.branchState()
		resolver.restoreBranchState(beforeBranches)
		if value.Else != nil {
			resolver.walkStatement(control, value.Else)
		}
		resolver.restoreBranchState(resolver.intersectGoBranchStates(thenState, resolver.branchState()))
	case *ast.ForStmt:
		control := resolver.newScope(scope)
		if value.Init != nil {
			resolver.walkStatement(control, value.Init)
		}
		resolver.walkExpressionCalls(control, value.Cond)
		resolver.walkStatements(resolver.newScope(control), value.Body.List)
		if value.Post != nil {
			resolver.walkStatement(control, value.Post)
		}
	case *ast.RangeStmt:
		control := resolver.newScope(scope)
		resolver.walkExpressionCalls(control, value.X)
		beforeLoop := resolver.branchState()
		rangeBindings := make(map[*goSymbol]goSymbolBindingState)
		if value.Tok != token.DEFINE {
			for _, expression := range []ast.Expr{value.Key, value.Value} {
				if identifier, ok := expression.(*ast.Ident); ok {
					if symbol := control.lookup(identifier.Name); symbol != nil {
						rangeBindings[symbol] = snapshotGoSymbolBinding(symbol)
					}
				}
			}
		}
		resolver.bindRange(control, value)
		resolver.walkStatements(resolver.newScope(control), value.Body.List)
		for symbol, binding := range rangeBindings {
			restoreGoSymbolBinding(symbol, binding)
		}
		resolver.restoreBranchState(beforeLoop)
	case *ast.SwitchStmt:
		control := resolver.newScope(scope)
		if value.Init != nil {
			resolver.walkStatement(control, value.Init)
		}
		resolver.walkExpressionCalls(control, value.Tag)
		resolver.walkSwitchBranches(control, value.Body.List, goBranchesHaveDefault(value.Body.List))
	case *ast.TypeSwitchStmt:
		control := resolver.newScope(scope)
		if value.Init != nil {
			resolver.walkStatement(control, value.Init)
		}
		if value.Assign != nil {
			resolver.walkStatement(control, value.Assign)
		}
		resolver.walkExclusiveBranches(control, value.Body.List, goBranchesHaveDefault(value.Body.List))
	case *ast.SelectStmt:
		control := resolver.newScope(scope)
		resolver.walkExclusiveBranches(control, value.Body.List, goBranchesHaveDefault(value.Body.List))
	case *ast.CaseClause:
		child := resolver.newScope(scope)
		for _, expression := range value.List {
			resolver.walkExpressionCalls(child, expression)
		}
		resolver.walkStatements(child, value.Body)
	case *ast.CommClause:
		child := resolver.newScope(scope)
		if value.Comm != nil {
			resolver.walkStatement(child, value.Comm)
		}
		resolver.walkStatements(child, value.Body)
	case *ast.DeferStmt:
		resolver.walkCall(scope, value.Call)
	case *ast.GoStmt:
		resolver.walkCall(scope, value.Call)
	case *ast.LabeledStmt:
		resolver.walkStatement(scope, value.Stmt)
	case *ast.SendStmt:
		resolver.walkExpressionCalls(scope, value.Chan)
		resolver.walkExpressionCalls(scope, value.Value)
	}
}

func (resolver *goMutationResolver) handleDeclaration(scope *goScope, declaration ast.Decl) {
	general, ok := declaration.(*ast.GenDecl)
	if !ok {
		return
	}
	for _, rawSpec := range general.Specs {
		spec, ok := rawSpec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for _, expression := range spec.Values {
			resolver.walkExpressionCalls(scope, expression)
		}
		for position, name := range spec.Names {
			var initial ast.Expr
			if position < len(spec.Values) {
				initial = spec.Values[position]
			} else if len(spec.Values) == 1 {
				initial = spec.Values[0]
			}
			shape := resolver.typeShape(spec.Type, nil)
			if spec.Type == nil {
				shape = resolver.expressionShape(scope, initial)
			}
			symbol := resolver.declare(scope, name.Name, goSymbolLocal, shape, originLocal, targetLocal, resolver.line(name.Pos()))
			resolver.setInitializer(scope, symbol, initial)
		}
	}
}

func (resolver *goMutationResolver) bindAssignment(scope *goScope, assignment *ast.AssignStmt) {
	for position, lhs := range assignment.Lhs {
		var rhs ast.Expr
		if position < len(assignment.Rhs) {
			rhs = assignment.Rhs[position]
		} else if len(assignment.Rhs) == 1 {
			rhs = assignment.Rhs[0]
		}
		if selector, ok := lhs.(*ast.SelectorExpr); ok {
			if assignment.Tok == token.ASSIGN {
				resolver.bindSelectorField(scope, selector, rhs)
			}
			continue
		}
		identifier, ok := lhs.(*ast.Ident)
		if !ok || identifier.Name == "_" {
			continue
		}
		if assignment.Tok == token.DEFINE {
			if scope.symbols[identifier.Name] != nil {
				resolver.setInitializer(scope, scope.symbols[identifier.Name], rhs)
				continue
			}
			shape := resolver.expressionShape(scope, rhs)
			origin := originLocal
			if _, ok := rhs.(*ast.CallExpr); ok && shape.Kind == goShapeUnknown {
				origin = originUnknown
			}
			symbol := resolver.declare(scope, identifier.Name, goSymbolLocal, shape, origin, targetLocal, resolver.line(identifier.Pos()))
			resolver.setInitializer(scope, symbol, rhs)
			continue
		}
		if symbol := scope.lookup(identifier.Name); symbol != nil && symbol.Kind != goSymbolGlobal {
			resolver.setInitializer(scope, symbol, rhs)
		}
	}
}

func (resolver *goMutationResolver) bindSelectorField(scope *goScope, selector *ast.SelectorExpr, expression ast.Expr) {
	if expression == nil {
		return
	}
	path := resolver.resolveExpression(scope, selector)
	if path.bindingKey == "" || path.packageName || path.crossedReference || path.symbol == nil {
		return
	}
	owner := resolver.resolveAlias(path.symbol)
	if owner == nil || owner.Kind == goSymbolGlobal || owner.Kind == goSymbolImport {
		return
	}
	resolver.nextID++
	binding := &goSymbol{
		ID:       resolver.nextID,
		Name:     selector.Sel.Name,
		Kind:     goSymbolLocal,
		Scope:    path.symbol.Scope,
		Shape:    path.shape,
		Origin:   originLocal,
		Target:   targetLocal,
		DeclLine: resolver.line(selector.Pos()),
	}
	resolver.symbols[binding.ID] = binding
	resolver.setInitializer(scope, binding, expression)
	resolver.fields[path.bindingKey] = binding
}

func (resolver *goMutationResolver) setInitializer(scope *goScope, symbol *goSymbol, expression ast.Expr) {
	if symbol == nil || expression == nil {
		return
	}
	path := resolver.resolveExpression(scope, expression)
	shape := resolver.expressionShape(scope, expression)
	aliasSource := resolver.resolveAlias(path.symbol)
	contentSource := resolver.resolveContentOwner(path.symbol)
	if shape.Kind != goShapeUnknown {
		symbol.Shape = shape
	}
	symbol.AliasOf = 0
	symbol.ContentOf = 0
	delete(resolver.closures, symbol.ID)
	if literal, ok := expression.(*ast.FuncLit); ok {
		resolver.closures[symbol.ID] = goClosureDefinition{literal: literal, scope: snapshotGoScope(scope)}
		symbol.Origin = originLocal
		return
	}
	if path.symbol != nil {
		if definition, ok := resolver.closures[path.symbol.ID]; ok {
			resolver.closures[symbol.ID] = definition
		}
	}
	if path.symbol != nil && path.symbol != symbol {
		if shape.referenceBacked() {
			if aliasSource != nil {
				symbol.AliasOf = aliasSource.ID
			} else {
				symbol.AliasOf = path.symbol.ID
			}
			symbol.Origin = originLocal
			return
		}
		if contentSource != nil {
			symbol.ContentOf = contentSource.ID
		} else {
			symbol.ContentOf = path.symbol.ID
		}
		symbol.Origin = originLocal
		return
	}
	if conversion, ok := expression.(*ast.CallExpr); ok {
		if conversionShape, isConversion := resolver.conversionShape(scope, conversion); isConversion && conversionShape.referenceBacked() && !resolver.conversionAllocatesFreshStorage(scope, conversion, conversionShape) {
			symbol.Origin = originUnknown
			return
		}
	}
	if _, ok := expression.(*ast.CallExpr); ok && shape.Kind == goShapeUnknown {
		symbol.Origin = originUnknown
	} else {
		symbol.Origin = originLocal
	}
}

func (resolver *goMutationResolver) bindRange(scope *goScope, statement *ast.RangeStmt) {
	shape := resolver.expressionShape(scope, statement.X)
	source := resolver.resolveExpression(scope, statement.X)
	valueShape := goReferenceShape{Kind: goShapeUnknown}
	if shape.Elem != nil {
		valueShape = *shape.Elem
	}
	bind := func(expression ast.Expr, itemShape goReferenceShape, inheritsContent bool) {
		identifier, ok := expression.(*ast.Ident)
		if !ok || identifier.Name == "_" {
			return
		}
		var symbol *goSymbol
		if statement.Tok == token.DEFINE {
			symbol = resolver.declare(scope, identifier.Name, goSymbolRange, itemShape, originLocal, targetLocal, resolver.line(identifier.Pos()))
		} else {
			symbol = scope.lookup(identifier.Name)
			if symbol == nil || symbol.Kind == goSymbolGlobal || symbol.Kind == goSymbolImport {
				return
			}
			symbol.Shape = itemShape
			symbol.Origin = originLocal
			symbol.Target = targetLocal
			symbol.AliasOf = 0
			symbol.ContentOf = 0
		}
		if inheritsContent && source.symbol != nil {
			if itemShape.referenceBacked() {
				symbol.AliasOf = source.symbol.ID
			} else {
				symbol.ContentOf = source.symbol.ID
			}
		}
	}
	bind(statement.Key, goReferenceShape{Kind: goShapeValue}, false)
	bind(statement.Value, valueShape, true)
}

func (resolver *goMutationResolver) recordEscapes(scope *goScope, assignment *ast.AssignStmt) {
	if assignment.Tok != token.ASSIGN {
		return
	}
	for position, lhs := range assignment.Lhs {
		if position >= len(assignment.Rhs) {
			break
		}
		rhsPath := resolver.resolveExpression(scope, assignment.Rhs[position])
		local := resolver.resolveAlias(rhsPath.symbol)
		if local == nil || local.Origin != originLocal || !local.Shape.referenceBacked() {
			continue
		}
		lhsPath := resolver.resolveExpression(scope, lhs)
		external := lhsPath.symbol != nil && lhsPath.symbol.Kind == goSymbolGlobal
		if !external {
			_, _, external = resolver.ownership(lhsPath, false)
		}
		if external {
			resolver.escapedAt[local.ID] = resolver.order
		}
	}
}

func (resolver *goMutationResolver) walkExpressionCalls(scope *goScope, expression ast.Expr) {
	if expression == nil {
		return
	}
	ast.Inspect(expression, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.CallExpr:
			resolver.walkCall(scope, value)
			return false
		}
		return true
	})
}

func (resolver *goMutationResolver) walkCall(scope *goScope, call *ast.CallExpr) {
	for _, argument := range call.Args {
		resolver.walkExpressionCalls(scope, argument)
	}
	if literal, ok := call.Fun.(*ast.FuncLit); ok {
		resolver.executeClosure(goClosureDefinition{literal: literal, scope: snapshotGoScope(scope)}, scope, call.Args, 0)
		return
	}
	line := resolver.line(call.Pos())
	if identifier, ok := call.Fun.(*ast.Ident); ok {
		if _, conversion := resolver.conversionShape(scope, call); conversion {
			return
		}
		if symbol := scope.lookup(identifier.Name); symbol != nil {
			if definition, exists := resolver.closures[symbol.ID]; exists {
				resolver.executeClosure(definition, scope, call.Args, symbol.ID)
			}
			return
		}
		if resolver.recordBuiltinMutation(scope, identifier.Name, call.Args, line) {
			return
		}
		if mutatingCallPattern.MatchString(identifier.Name) && !isConstructionOrHydrationCall(identifier.Name) {
			resolver.addUnresolved(line, "call", identifier.Name, "call target declaration could not be resolved")
		}
		return
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	path := resolver.resolveExpression(scope, selector.X)
	if path.packageName {
		return
	}
	callee := goCallName(call.Fun)
	effect := observableCallEffect(callee)
	if effect == "" {
		if !mutatingCallPattern.MatchString(callee) || isConstructionOrHydrationCall(callee) {
			return
		}
		effect = "shared_state"
	}
	target, origin, reportable := resolver.ownership(path, true)
	if reportable {
		resolver.addMutation(target, effect, origin, line, callee)
		return
	}
	if path.symbol == nil || path.symbol.Origin == originUnknown {
		resolver.addUnresolved(line, "call", firstNonEmptyString(path.unresolved, resolver.expressionRootName(selector.X)), "call target ownership could not be resolved")
	}
}

func (resolver *goMutationResolver) executeClosure(definition goClosureDefinition, callScope *goScope, arguments []ast.Expr, symbolID int) {
	if definition.literal == nil || definition.scope == nil {
		return
	}
	if symbolID != 0 {
		if resolver.active[symbolID] {
			return
		}
		resolver.active[symbolID] = true
		defer delete(resolver.active, symbolID)
	}
	closure := resolver.newScope(definition.scope)
	resolver.declareClosureParameters(callScope, closure, definition.literal.Type.Params, arguments)
	resolver.declareFieldList(closure, definition.literal.Type.Results, goSymbolResult, originLocal, targetLocal)
	resolver.walkStatements(closure, definition.literal.Body.List)
}

func (resolver *goMutationResolver) declareClosureParameters(outer *goScope, closure *goScope, fields *ast.FieldList, arguments []ast.Expr) {
	if fields == nil {
		return
	}
	argumentIndex := 0
	for _, field := range fields.List {
		shape := resolver.typeShape(field.Type, nil)
		for _, name := range field.Names {
			symbol := resolver.declare(closure, name.Name, goSymbolParameter, shape, originLocal, targetLocal, resolver.line(name.Pos()))
			if argumentIndex < len(arguments) {
				resolver.setInitializer(outer, symbol, arguments[argumentIndex])
			}
			argumentIndex++
		}
	}
}

func (resolver *goMutationResolver) recordBuiltinMutation(scope *goScope, name string, arguments []ast.Expr, line int) bool {
	mutatesFirstArgument := name == "append" || name == "copy" || name == "delete" || name == "clear"
	if !mutatesFirstArgument {
		return name == "make" || name == "new" || name == "len" || name == "cap" || name == "close" || name == "panic" || name == "recover" || name == "complex" || name == "real" || name == "imag" || name == "print" || name == "println"
	}
	if len(arguments) == 0 {
		return true
	}
	path := resolver.resolveExpression(scope, arguments[0])
	if path.shape.referenceBacked() {
		path.crossedReference = true
	}
	target, origin, reportable := resolver.ownership(path, false)
	if reportable {
		resolver.addMutation(target, "shared_state", origin, line, name)
	} else if path.symbol == nil || path.symbol.Origin == originUnknown {
		resolver.addUnresolved(line, "call", firstNonEmptyString(path.unresolved, resolver.expressionRootName(arguments[0])), "built-in mutation target could not be resolved")
	}
	return true
}
