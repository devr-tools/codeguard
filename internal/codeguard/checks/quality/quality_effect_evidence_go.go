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
	symbols map[int]goSymbolBindingState
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

func cloneGoSymbolBindings(symbols map[int]*goSymbol) map[int]goSymbolBindingState {
	bindings := make(map[int]goSymbolBindingState, len(symbols))
	for id, symbol := range symbols {
		bindings[id] = snapshotGoSymbolBinding(symbol)
	}
	return bindings
}

func cloneGoSymbolBindingStates(states map[int]goSymbolBindingState) map[int]goSymbolBindingState {
	clone := make(map[int]goSymbolBindingState, len(states))
	for id, state := range states {
		clone[id] = state
	}
	return clone
}

func goSymbolBindingStatesEqual(first goSymbolBindingState, second goSymbolBindingState) bool {
	return goReferenceShapesEqual(first.shape, second.shape) &&
		first.origin == second.origin && first.target == second.target &&
		first.aliasOf == second.aliasOf && first.contentOf == second.contentOf
}

func (resolver *goMutationResolver) branchState() goBranchState {
	return goBranchState{
		escapes: cloneGoEscapeState(resolver.escapedAt),
		fields:  cloneGoFieldBindings(resolver.fields),
		symbols: cloneGoSymbolBindings(resolver.symbols),
	}
}

func (resolver *goMutationResolver) restoreBranchState(state goBranchState) {
	resolver.escapedAt = cloneGoEscapeState(state.escapes)
	resolver.fields = cloneGoFieldBindings(state.fields)
	for id, binding := range state.symbols {
		if symbol := resolver.symbols[id]; symbol != nil {
			restoreGoSymbolBinding(symbol, binding)
		}
	}
}

func (resolver *goMutationResolver) mergeGoBranchStates(before goBranchState, states ...goBranchState) goBranchState {
	if len(states) == 0 {
		return before
	}
	merged := goBranchState{
		escapes: cloneGoEscapeState(states[0].escapes),
		fields:  cloneGoFieldBindings(states[0].fields),
		symbols: cloneGoSymbolBindingStates(before.symbols),
	}
	for _, state := range states[1:] {
		merged.escapes = intersectGoEscapeStates(merged.escapes, state.escapes)
		merged.fields = resolver.intersectGoFieldBindings(merged.fields, state.fields)
	}
	for id := range before.symbols {
		candidate, exists := states[0].symbols[id]
		if !exists {
			continue
		}
		equivalent := true
		for _, state := range states[1:] {
			other, ok := state.symbols[id]
			if !ok || !goSymbolBindingStatesEqual(candidate, other) {
				equivalent = false
				break
			}
		}
		if equivalent {
			merged.symbols[id] = candidate
		}
	}
	return merged
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
	resolver.restoreBranchState(resolver.mergeGoBranchStates(before, states...))
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
	resolver.restoreBranchState(resolver.mergeGoBranchStates(before, states...))
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
