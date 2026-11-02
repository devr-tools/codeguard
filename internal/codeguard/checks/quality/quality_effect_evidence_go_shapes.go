package quality

import (
	"go/ast"
)

// goTypeShapes derives reference shapes from type expressions using the
// declarations of the package being analyzed.
type goTypeShapes struct {
	pkg *goPackageInfo
}

func (shapes goTypeShapes) typeShape(expression ast.Expr, visiting map[string]bool) goReferenceShape {
	if expression == nil {
		return goReferenceShape{Kind: goShapeUnknown}
	}
	switch value := expression.(type) {
	case *ast.ParenExpr:
		return shapes.typeShape(value.X, visiting)
	case *ast.StarExpr:
		element := shapes.typeShape(value.X, visiting)
		return goReferenceShape{Kind: goShapePointer, Elem: &element}
	case *ast.MapType:
		element := shapes.typeShape(value.Value, visiting)
		return goReferenceShape{Kind: goShapeMap, Elem: &element}
	case *ast.ArrayType:
		element := shapes.typeShape(value.Elt, visiting)
		kind := goShapeArray
		if value.Len == nil {
			kind = goShapeSlice
		}
		return goReferenceShape{Kind: kind, Elem: &element}
	case *ast.Ellipsis:
		element := shapes.typeShape(value.Elt, visiting)
		return goReferenceShape{Kind: goShapeSlice, Elem: &element}
	case *ast.IndexExpr:
		return shapes.typeShape(value.X, visiting)
	case *ast.IndexListExpr:
		return shapes.typeShape(value.X, visiting)
	case *ast.InterfaceType:
		return goReferenceShape{Kind: goShapeInterface}
	case *ast.StructType:
		return shapes.structShape(value, visiting)
	case *ast.Ident:
		return shapes.namedTypeShape(value, visiting)
	case *ast.ChanType, *ast.FuncType:
		return goReferenceShape{Kind: goShapeValue}
	default:
		return goReferenceShape{Kind: goShapeUnknown}
	}
}

func (shapes goTypeShapes) isTypeExpression(scope *goScope, expression ast.Expr) bool {
	switch value := expression.(type) {
	case *ast.ArrayType, *ast.MapType, *ast.InterfaceType, *ast.StructType:
		return true
	case *ast.ParenExpr:
		return shapes.isTypeExpression(scope, value.X)
	case *ast.StarExpr:
		return shapes.isTypeExpression(scope, value.X)
	case *ast.IndexExpr:
		return shapes.isTypeExpression(scope, value.X)
	case *ast.IndexListExpr:
		return shapes.isTypeExpression(scope, value.X)
	case *ast.Ident:
		return (scope == nil || scope.lookup(value.Name) == nil) && shapes.pkg != nil && shapes.pkg.types[value.Name] != nil
	default:
		return false
	}
}

func (shapes goTypeShapes) conversionShape(scope *goScope, call *ast.CallExpr) (goReferenceShape, bool) {
	if call == nil || len(call.Args) != 1 {
		return goReferenceShape{}, false
	}
	if !shapes.isTypeExpression(scope, call.Fun) {
		return goReferenceShape{}, false
	}
	return shapes.typeShape(call.Fun, nil), true
}

func (shapes goTypeShapes) underlyingScalar(name string) string {
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
		if shapes.pkg == nil {
			return ""
		}
		declaration, ok := shapes.pkg.types[name].(*ast.Ident)
		if !ok {
			return ""
		}
		name = declaration.Name
	}
	return ""
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

// structShape records the shape of every named field of a struct type.
func (shapes goTypeShapes) structShape(structType *ast.StructType, visiting map[string]bool) goReferenceShape {
	shape := goReferenceShape{Kind: goShapeValue, Fields: make(map[string]goReferenceShape)}
	if structType.Fields == nil {
		return shape
	}
	for _, field := range structType.Fields.List {
		fieldShape := shapes.typeShape(field.Type, visiting)
		for _, name := range field.Names {
			shape.Fields[name.Name] = fieldShape
		}
	}
	return shape
}

// namedTypeShape expands a named type through its package declaration,
// guarding against recursive type definitions with visiting.
func (shapes goTypeShapes) namedTypeShape(identifier *ast.Ident, visiting map[string]bool) goReferenceShape {
	if identifier.Name == "any" || identifier.Name == "error" {
		return goReferenceShape{Kind: goShapeInterface, Name: identifier.Name}
	}
	if shapes.pkg == nil || shapes.pkg.types[identifier.Name] == nil {
		return goReferenceShape{Kind: goShapeValue, Name: identifier.Name}
	}
	if visiting == nil {
		visiting = make(map[string]bool)
	}
	if visiting[identifier.Name] {
		return goReferenceShape{Kind: goShapeValue, Name: identifier.Name}
	}
	visiting[identifier.Name] = true
	shape := shapes.typeShape(shapes.pkg.types[identifier.Name], visiting)
	delete(visiting, identifier.Name)
	shape.Name = identifier.Name
	return shape
}
