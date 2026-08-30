package quality

import (
	"go/ast"
	"go/token"
)

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
					resolver.recordAssignmentMutation(scope, expression, line, resolver.expressionRootName(expression))
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
		resolver.restoreBranchState(resolver.mergeGoBranchStates(beforeBranches, thenState, resolver.branchState()))
	case *ast.ForStmt:
		control := resolver.newScope(scope)
		if value.Init != nil {
			resolver.walkStatement(control, value.Init)
		}
		resolver.walkExpressionCalls(control, value.Cond)
		beforeLoop := resolver.branchState()
		resolver.walkStatements(resolver.newScope(control), value.Body.List)
		if value.Post != nil {
			resolver.walkStatement(control, value.Post)
		}
		resolver.restoreBranchState(beforeLoop)
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
		if mutatingCallPattern.MatchString(identifier.Name) {
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
		if !mutatingCallPattern.MatchString(callee) {
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
