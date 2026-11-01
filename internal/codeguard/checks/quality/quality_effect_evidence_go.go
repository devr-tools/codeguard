package quality

import (
	"go/ast"
	"go/token"
	"path/filepath"
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

// goMutationResolver walks a Go function body in execution order and records
// mutations of state the function does not own. Concerns are delegated:
// shapes derives type shapes, table owns scopes and symbols, exprs resolves
// expressions, flow tracks path-sensitive bindings, and evidence collects
// the results.
type goMutationResolver struct {
	fn       precisionFunction
	pkg      *goPackageInfo
	fset     *token.FileSet
	shapes   goTypeShapes
	table    *goSymbolTable
	flow     *goFlowState
	exprs    goExpressionResolver
	evidence *goEvidenceRecorder
	order    int
	closures map[int]goClosureDefinition
	active   map[int]bool
}

func goFunctionMutationEvidence(fn precisionFunction) mutationAnalysis {
	if fn.GoDecl == nil || fn.GoDecl.Body == nil {
		return mutationAnalysis{}
	}
	shapes := goTypeShapes{pkg: fn.GoPackage}
	table := &goSymbolTable{symbols: make(map[int]*goSymbol)}
	flow := &goFlowState{table: table, escapedAt: make(map[int]int), fields: make(map[string]*goSymbol)}
	resolver := &goMutationResolver{
		fn:       fn,
		pkg:      fn.GoPackage,
		fset:     fn.GoFSet,
		shapes:   shapes,
		table:    table,
		flow:     flow,
		exprs:    goExpressionResolver{shapes: shapes, flow: flow},
		evidence: &goEvidenceRecorder{seen: make(map[string]struct{}), unseen: make(map[string]struct{})},
		closures: make(map[int]goClosureDefinition),
		active:   make(map[int]bool),
	}
	packageScope := resolver.table.newScope(nil)
	resolver.declarePackageSymbols(packageScope)
	functionScope := resolver.table.newScope(packageScope)
	resolver.declareFunctionFields(functionScope, fn.GoDecl)
	resolver.walkStatements(functionScope, fn.GoDecl.Body.List)
	return resolver.evidence.analysis
}

func (resolver *goMutationResolver) declarePackageSymbols(scope *goScope) {
	if resolver.pkg != nil {
		for name, variable := range resolver.pkg.variables {
			shape := resolver.shapes.typeShape(variable.typ, nil)
			if variable.typ == nil {
				shape = resolver.exprs.expressionShape(scope, variable.init)
			}
			resolver.table.declare(scope, name, goSymbolGlobal, shape, originShared, targetGlobal, variable.line)
		}
		for name := range resolver.pkg.importsByFile[filepath.Clean(resolver.fn.GoFile)] {
			resolver.table.declare(scope, name, goSymbolImport, goReferenceShape{Kind: goShapePackage, Name: name}, "", "", 0)
		}
		if resolver.fn.GoFile == "" && len(resolver.pkg.importsByFile) == 1 {
			for _, imports := range resolver.pkg.importsByFile {
				for name := range imports {
					resolver.table.declare(scope, name, goSymbolImport, goReferenceShape{Kind: goShapePackage, Name: name}, "", "", 0)
				}
			}
		}
		return
	}
	for name := range resolver.fn.ProvenGlobals {
		resolver.table.declare(scope, name, goSymbolGlobal, goReferenceShape{Kind: goShapeUnknown}, originShared, targetGlobal, resolver.fn.StartLine)
	}
}

func (resolver *goMutationResolver) declareFunctionFields(scope *goScope, declaration *ast.FuncDecl) {
	if declaration.Recv != nil {
		for _, field := range declaration.Recv.List {
			shape := resolver.shapes.typeShape(field.Type, nil)
			for _, name := range field.Names {
				resolver.table.declare(scope, name.Name, goSymbolReceiver, shape, originCaller, targetReceiver, resolver.line(name.Pos()))
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
		shape := resolver.shapes.typeShape(field.Type, nil)
		for _, name := range field.Names {
			resolver.table.declare(scope, name.Name, kind, shape, origin, target, resolver.line(name.Pos()))
		}
	}
}

func (resolver *goMutationResolver) line(position token.Pos) int {
	if resolver.fset == nil {
		return resolver.fn.StartLine
	}
	return resolver.fset.Position(position).Line
}

func (resolver *goMutationResolver) ownership(path goExpressionPath, call bool) (string, string, bool) {
	if path.symbol == nil || path.packageName {
		return "", "", false
	}
	symbol := resolver.table.resolveAlias(path.symbol)
	if symbol == nil {
		return "", "", false
	}
	if path.crossedReference && path.symbol.ContentOf != 0 {
		if owner := resolver.table.resolveContentOwner(path.symbol); owner != nil {
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
		root := resolver.table.resolveAlias(path.symbol)
		if root != nil && resolver.flow.escapedAt[root.ID] > 0 && resolver.order > resolver.flow.escapedAt[root.ID] {
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

func (resolver *goMutationResolver) recordAssignmentMutation(scope *goScope, expression ast.Expr, line int, detail string) {
	path := resolver.exprs.resolveExpression(scope, expression)
	if path.packageName {
		return
	}
	target, origin, reportable := resolver.ownership(path, false)
	if reportable {
		resolver.evidence.addMutation(target, "shared_state", origin, line, detail)
		return
	}
	if path.symbol == nil || path.symbol.Origin == originUnknown || (path.symbol.Origin == originCaller && path.shape.Kind == goShapeUnknown) {
		resolver.evidence.addUnresolved(line, "assignment", firstNonEmptyString(path.unresolved, goExpressionRootName(expression)), "symbol ownership or reference shape could not be resolved")
	}
}
