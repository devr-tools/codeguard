package quality

import (
	"go/ast"
	"go/token"
)

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
			initial := goValueAt(spec.Values, position)
			shape := resolver.shapes.typeShape(spec.Type, nil)
			if spec.Type == nil {
				shape = resolver.exprs.expressionShape(scope, initial)
			}
			symbol := resolver.table.declare(scope, name.Name, goSymbolLocal, shape, originLocal, targetLocal, resolver.line(name.Pos()))
			resolver.setInitializer(scope, symbol, initial)
		}
	}
}

func (resolver *goMutationResolver) bindAssignment(scope *goScope, assignment *ast.AssignStmt) {
	for position, lhs := range assignment.Lhs {
		rhs := goValueAt(assignment.Rhs, position)
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
			shape := resolver.exprs.expressionShape(scope, rhs)
			origin := originLocal
			if _, ok := rhs.(*ast.CallExpr); ok && shape.Kind == goShapeUnknown {
				origin = originUnknown
			}
			symbol := resolver.table.declare(scope, identifier.Name, goSymbolLocal, shape, origin, targetLocal, resolver.line(identifier.Pos()))
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
	path := resolver.exprs.resolveExpression(scope, selector)
	if path.bindingKey == "" || path.packageName || path.crossedReference || path.symbol == nil {
		return
	}
	owner := resolver.table.resolveAlias(path.symbol)
	if owner == nil || owner.Kind == goSymbolGlobal || owner.Kind == goSymbolImport {
		return
	}
	binding := resolver.table.allocate(&goSymbol{
		Name:     selector.Sel.Name,
		Kind:     goSymbolLocal,
		Scope:    path.symbol.Scope,
		Shape:    path.shape,
		Origin:   originLocal,
		Target:   targetLocal,
		DeclLine: resolver.line(selector.Pos()),
	})
	resolver.setInitializer(scope, binding, expression)
	resolver.flow.fields[path.bindingKey] = binding
}

func (resolver *goMutationResolver) setInitializer(scope *goScope, symbol *goSymbol, expression ast.Expr) {
	if symbol == nil || expression == nil {
		return
	}
	path := resolver.exprs.resolveExpression(scope, expression)
	shape := resolver.exprs.expressionShape(scope, expression)
	aliasSource := resolver.table.resolveAlias(path.symbol)
	contentSource := resolver.table.resolveContentOwner(path.symbol)
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
		if conversionShape, isConversion := resolver.shapes.conversionShape(scope, conversion); isConversion && conversionShape.referenceBacked() && !resolver.exprs.conversionAllocatesFreshStorage(scope, conversion, conversionShape) {
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
	shape := resolver.exprs.expressionShape(scope, statement.X)
	source := resolver.exprs.resolveExpression(scope, statement.X)
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
			symbol = resolver.table.declare(scope, identifier.Name, goSymbolRange, itemShape, originLocal, targetLocal, resolver.line(identifier.Pos()))
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
		rhsPath := resolver.exprs.resolveExpression(scope, assignment.Rhs[position])
		local := resolver.table.resolveAlias(rhsPath.symbol)
		if local == nil || local.Origin != originLocal || !local.Shape.referenceBacked() {
			continue
		}
		lhsPath := resolver.exprs.resolveExpression(scope, lhs)
		external := lhsPath.symbol != nil && lhsPath.symbol.Kind == goSymbolGlobal
		if !external {
			_, _, external = resolver.ownership(lhsPath, false)
		}
		if external {
			resolver.flow.escapedAt[local.ID] = resolver.order
		}
	}
}
