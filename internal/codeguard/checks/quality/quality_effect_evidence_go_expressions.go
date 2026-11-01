package quality

import (
	"go/ast"
	"go/token"
	"strconv"
)

// goExpressionResolver resolves expressions to the symbol they are rooted
// in and the reference shape they evaluate to.
type goExpressionResolver struct {
	shapes goTypeShapes
	flow   *goFlowState
}

func (exprs goExpressionResolver) expressionShape(scope *goScope, expression ast.Expr) goReferenceShape {
	if expression == nil {
		return goReferenceShape{Kind: goShapeUnknown}
	}
	switch value := expression.(type) {
	case *ast.CompositeLit:
		return exprs.shapes.typeShape(value.Type, nil)
	case *ast.UnaryExpr:
		if value.Op == token.AND {
			element := exprs.expressionShape(scope, value.X)
			return goReferenceShape{Kind: goShapePointer, Elem: &element}
		}
		return exprs.expressionShape(scope, value.X)
	case *ast.Ident, *ast.SelectorExpr, *ast.IndexExpr, *ast.SliceExpr, *ast.ParenExpr, *ast.StarExpr, *ast.TypeAssertExpr:
		return exprs.resolveExpression(scope, expression).shape
	case *ast.CallExpr:
		if identifier, ok := value.Fun.(*ast.Ident); ok {
			switch identifier.Name {
			case "append":
				if len(value.Args) > 0 {
					return exprs.expressionShape(scope, value.Args[0])
				}
			case "make":
				if len(value.Args) > 0 {
					return exprs.shapes.typeShape(value.Args[0], nil)
				}
			case "new":
				if len(value.Args) > 0 {
					element := exprs.shapes.typeShape(value.Args[0], nil)
					return goReferenceShape{Kind: goShapePointer, Elem: &element}
				}
			}
		}
		if shape, ok := exprs.shapes.conversionShape(scope, value); ok {
			return shape
		}
		return goReferenceShape{Kind: goShapeUnknown}
	case *ast.FuncLit:
		return goReferenceShape{Kind: goShapeValue}
	default:
		return goReferenceShape{Kind: goShapeValue}
	}
}

func (exprs goExpressionResolver) freshConversionOperand(scope *goScope, expression ast.Expr) bool {
	switch value := expression.(type) {
	case *ast.ParenExpr:
		return exprs.freshConversionOperand(scope, value.X)
	case *ast.Ident:
		return value.Name == "nil"
	case *ast.BasicLit, *ast.CompositeLit:
		return true
	case *ast.UnaryExpr:
		return value.Op == token.AND && exprs.freshConversionOperand(scope, value.X)
	case *ast.CallExpr:
		if identifier, ok := value.Fun.(*ast.Ident); ok && (identifier.Name == "make" || identifier.Name == "new") {
			return true
		}
		shape, conversion := exprs.shapes.conversionShape(scope, value)
		return conversion && exprs.conversionAllocatesFreshStorage(scope, value, shape)
	default:
		return false
	}
}

func (exprs goExpressionResolver) conversionAllocatesFreshStorage(scope *goScope, call *ast.CallExpr, target goReferenceShape) bool {
	if call == nil || len(call.Args) != 1 {
		return false
	}
	if exprs.freshConversionOperand(scope, call.Args[0]) {
		return true
	}
	if target.Kind != goShapeSlice || target.Elem == nil {
		return false
	}
	source := exprs.expressionShape(scope, call.Args[0])
	if exprs.shapes.underlyingScalar(source.Name) != "string" {
		return false
	}
	switch exprs.shapes.underlyingScalar(target.Elem.Name) {
	case "uint8", "int32":
		return true
	default:
		return false
	}
}

func (exprs goExpressionResolver) resolveExpression(scope *goScope, expression ast.Expr) goExpressionPath {
	switch value := expression.(type) {
	case *ast.Ident:
		return resolveGoIdentifier(scope, value)
	case *ast.ParenExpr:
		return exprs.resolveExpression(scope, value.X)
	case *ast.StarExpr:
		return dereferenceGoPath(exprs.resolveExpression(scope, value.X), value.X)
	case *ast.SelectorExpr:
		return selectGoField(exprs.resolveExpression(scope, value.X), value.Sel.Name, exprs.flow.fields)
	case *ast.IndexExpr:
		return indexGoPath(exprs.resolveExpression(scope, value.X))
	case *ast.SliceExpr:
		path := exprs.resolveExpression(scope, value.X)
		// A slice expression retains the backing storage and ownership of its
		// source. Keep the root symbol so append(input[:0], ...) cannot be
		// mistaken for a fresh local copy.
		if path.shape.Kind != goShapeSlice {
			path.shape = exprs.expressionShape(scope, value.X)
		}
		return path
	case *ast.CallExpr:
		return exprs.resolveCall(scope, value)
	case *ast.TypeAssertExpr:
		path := exprs.resolveExpression(scope, value.X)
		if value.Type != nil {
			path = assertGoPath(path, exprs.shapes.typeShape(value.Type, nil))
		}
		return path
	default:
		return goExpressionPath{shape: exprs.expressionShape(scope, expression), unresolved: goExpressionRootName(expression)}
	}
}

// resolveCall resolves append (which keeps its first argument's storage),
// type conversions (which keep the operand's storage unless they allocate),
// and otherwise reports the call's root as unresolved.
func (exprs goExpressionResolver) resolveCall(scope *goScope, call *ast.CallExpr) goExpressionPath {
	if identifier, ok := call.Fun.(*ast.Ident); ok && identifier.Name == "append" && len(call.Args) > 0 {
		path := exprs.resolveExpression(scope, call.Args[0])
		path.shape = exprs.expressionShape(scope, call)
		return path
	}
	if shape, ok := exprs.shapes.conversionShape(scope, call); ok {
		if exprs.conversionAllocatesFreshStorage(scope, call, shape) {
			return goExpressionPath{shape: shape}
		}
		path := exprs.resolveExpression(scope, call.Args[0])
		path.shape = shape
		return path
	}
	return goExpressionPath{shape: exprs.expressionShape(scope, call), unresolved: goExpressionRootName(call)}
}

func resolveGoIdentifier(scope *goScope, identifier *ast.Ident) goExpressionPath {
	symbol := scope.lookup(identifier.Name)
	if symbol == nil {
		return goExpressionPath{shape: goReferenceShape{Kind: goShapeUnknown}, unresolved: identifier.Name}
	}
	return goExpressionPath{symbol: symbol, shape: symbol.Shape, packageName: symbol.Kind == goSymbolImport, bindingKey: strconv.Itoa(symbol.ID)}
}

// dereferenceGoPath follows a pointer dereference, crossing into the
// pointee's storage.
func dereferenceGoPath(path goExpressionPath, operand ast.Expr) goExpressionPath {
	if path.shape.Kind == goShapePointer && path.shape.Elem != nil {
		path.shape = *path.shape.Elem
		path.crossedReference = true
	} else if path.shape.Kind == goShapeUnknown {
		path.unresolved = firstNonEmptyString(path.unresolved, goExpressionRootName(operand))
	}
	return path
}

// selectGoField follows a field selection. A field previously bound to a
// known symbol resolves to that symbol; selecting through a pointer crosses
// into shared storage.
func selectGoField(path goExpressionPath, name string, fields map[string]*goSymbol) goExpressionPath {
	if path.packageName {
		return path
	}
	if path.bindingKey != "" {
		path.bindingKey += "." + name
		if binding := fields[path.bindingKey]; binding != nil {
			return goExpressionPath{symbol: binding, shape: binding.Shape, bindingKey: path.bindingKey}
		}
	}
	if path.shape.Kind == goShapePointer {
		path.crossedReference = true
		if path.shape.Elem != nil {
			path.shape = *path.shape.Elem
		}
	}
	if field, ok := path.shape.Fields[name]; ok {
		path.shape = field
	} else if path.shape.Kind != goShapeInterface {
		path.shape = goReferenceShape{Kind: goShapeUnknown}
	}
	return path
}

// indexGoPath follows an index expression; indexing a map or slice crosses
// into its backing storage.
func indexGoPath(path goExpressionPath) goExpressionPath {
	if path.shape.Kind == goShapeMap || path.shape.Kind == goShapeSlice {
		path.crossedReference = true
	}
	if path.shape.Elem != nil {
		path.shape = *path.shape.Elem
	} else {
		path.shape = goReferenceShape{Kind: goShapeUnknown}
	}
	return path
}

// assertGoPath applies a type assertion; asserting to a reference-backed type
// crosses into shared storage.
func assertGoPath(path goExpressionPath, asserted goReferenceShape) goExpressionPath {
	path.shape = asserted
	if asserted.referenceBacked() {
		path.crossedReference = true
	}
	return path
}

func goExpressionRootName(expression ast.Expr) string {
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
