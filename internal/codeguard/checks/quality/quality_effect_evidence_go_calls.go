package quality

import (
	"go/ast"
)

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
		if _, conversion := resolver.shapes.conversionShape(scope, call); conversion {
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
			resolver.evidence.addUnresolved(line, "call", identifier.Name, "call target declaration could not be resolved")
		}
		return
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	path := resolver.exprs.resolveExpression(scope, selector.X)
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
		resolver.evidence.addMutation(target, effect, origin, line, callee)
		return
	}
	if path.symbol == nil || path.symbol.Origin == originUnknown {
		resolver.evidence.addUnresolved(line, "call", firstNonEmptyString(path.unresolved, goExpressionRootName(selector.X)), "call target ownership could not be resolved")
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
	closure := resolver.table.newScope(definition.scope)
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
		shape := resolver.shapes.typeShape(field.Type, nil)
		for _, name := range field.Names {
			symbol := resolver.table.declare(closure, name.Name, goSymbolParameter, shape, originLocal, targetLocal, resolver.line(name.Pos()))
			if argumentIndex < len(arguments) {
				resolver.setInitializer(outer, symbol, arguments[argumentIndex])
			}
			argumentIndex++
		}
	}
}

// goFirstArgumentMutatingBuiltins mutate the storage of their first argument.
var goFirstArgumentMutatingBuiltins = map[string]bool{"append": true, "copy": true, "delete": true, "clear": true}

// goNonMutatingBuiltins are built-ins known not to mutate caller-visible
// state, so calls to them are neither evidence nor unresolved.
var goNonMutatingBuiltins = map[string]bool{
	"make": true, "new": true, "len": true, "cap": true, "close": true, "panic": true,
	"recover": true, "complex": true, "real": true, "imag": true, "print": true, "println": true,
}

func (resolver *goMutationResolver) recordBuiltinMutation(scope *goScope, name string, arguments []ast.Expr, line int) bool {
	if !goFirstArgumentMutatingBuiltins[name] {
		return goNonMutatingBuiltins[name]
	}
	if len(arguments) == 0 {
		return true
	}
	path := resolver.exprs.resolveExpression(scope, arguments[0])
	if path.shape.referenceBacked() {
		path.crossedReference = true
	}
	target, origin, reportable := resolver.ownership(path, false)
	if reportable {
		resolver.evidence.addMutation(target, "shared_state", origin, line, name)
	} else if path.symbol == nil || path.symbol.Origin == originUnknown {
		resolver.evidence.addUnresolved(line, "call", firstNonEmptyString(path.unresolved, goExpressionRootName(arguments[0])), "built-in mutation target could not be resolved")
	}
	return true
}
