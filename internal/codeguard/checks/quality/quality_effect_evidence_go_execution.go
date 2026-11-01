package quality

import (
	"go/ast"
	"go/token"
)

func (resolver *goMutationResolver) walkStatements(scope *goScope, statements []ast.Stmt) {
	for _, statement := range statements {
		resolver.walkStatement(scope, statement)
	}
}

// walkStatement advances the execution order and dispatches one statement
// to the walker for its kind: control flow (which forks and merges binding
// state) or straight-line statements.
func (resolver *goMutationResolver) walkStatement(scope *goScope, statement ast.Stmt) {
	resolver.order++
	if resolver.walkControlFlow(scope, statement) {
		return
	}
	resolver.walkStraightLine(scope, statement)
}

// walkControlFlow walks statements that introduce scopes or branches and
// reports whether statement was one of them.
func (resolver *goMutationResolver) walkControlFlow(scope *goScope, statement ast.Stmt) bool {
	switch value := statement.(type) {
	case *ast.BlockStmt:
		resolver.walkStatements(resolver.table.newScope(scope), value.List)
	case *ast.IfStmt:
		resolver.walkIf(scope, value)
	case *ast.ForStmt:
		resolver.walkFor(scope, value)
	case *ast.RangeStmt:
		resolver.walkRange(scope, value)
	case *ast.SwitchStmt:
		resolver.walkSwitch(scope, value)
	case *ast.TypeSwitchStmt:
		resolver.walkTypeSwitch(scope, value)
	case *ast.SelectStmt:
		control := resolver.table.newScope(scope)
		resolver.walkExclusiveBranches(control, value.Body.List, goBranchesHaveDefault(value.Body.List))
	case *ast.CaseClause:
		resolver.walkCaseClause(scope, value)
	case *ast.CommClause:
		resolver.walkCommClause(scope, value)
	case *ast.LabeledStmt:
		resolver.walkStatement(scope, value.Stmt)
	default:
		return false
	}
	return true
}

// walkStraightLine walks declarations, assignments, and statements that only
// evaluate expressions.
func (resolver *goMutationResolver) walkStraightLine(scope *goScope, statement ast.Stmt) {
	switch value := statement.(type) {
	case *ast.DeclStmt:
		resolver.handleDeclaration(scope, value.Decl)
	case *ast.AssignStmt:
		resolver.walkAssignment(scope, value)
	case *ast.IncDecStmt:
		resolver.recordAssignmentMutation(scope, value.X, resolver.line(value.Pos()), goExpressionRootName(value.X))
	case *ast.ExprStmt:
		resolver.walkExpressionCalls(scope, value.X)
	case *ast.ReturnStmt:
		for _, expression := range value.Results {
			resolver.walkExpressionCalls(scope, expression)
		}
	case *ast.DeferStmt:
		resolver.walkCall(scope, value.Call)
	case *ast.GoStmt:
		resolver.walkCall(scope, value.Call)
	case *ast.SendStmt:
		resolver.walkExpressionCalls(scope, value.Chan)
		resolver.walkExpressionCalls(scope, value.Value)
	}
}

func (resolver *goMutationResolver) walkAssignment(scope *goScope, assignment *ast.AssignStmt) {
	for _, expression := range assignment.Rhs {
		resolver.walkExpressionCalls(scope, expression)
	}
	line := resolver.line(assignment.Pos())
	if assignment.Tok != token.DEFINE {
		for _, expression := range assignment.Lhs {
			switch expression.(type) {
			case *ast.SelectorExpr, *ast.IndexExpr, *ast.StarExpr, *ast.Ident:
				resolver.recordAssignmentMutation(scope, expression, line, goExpressionRootName(expression))
			}
		}
	}
	resolver.recordEscapes(scope, assignment)
	resolver.bindAssignment(scope, assignment)
}

func (resolver *goMutationResolver) walkIf(scope *goScope, statement *ast.IfStmt) {
	control := resolver.table.newScope(scope)
	if statement.Init != nil {
		resolver.walkStatement(control, statement.Init)
	}
	resolver.walkExpressionCalls(control, statement.Cond)
	beforeBranches := resolver.flow.snapshot()
	resolver.flow.restore(beforeBranches)
	resolver.walkStatements(resolver.table.newScope(control), statement.Body.List)
	thenState := resolver.flow.snapshot()
	resolver.flow.restore(beforeBranches)
	if statement.Else != nil {
		resolver.walkStatement(control, statement.Else)
	}
	resolver.flow.restore(resolver.flow.merge(beforeBranches, thenState, resolver.flow.snapshot()))
}

// walkFor walks one iteration of a loop and then discards its binding
// changes, since the body may run zero times.
func (resolver *goMutationResolver) walkFor(scope *goScope, statement *ast.ForStmt) {
	control := resolver.table.newScope(scope)
	if statement.Init != nil {
		resolver.walkStatement(control, statement.Init)
	}
	resolver.walkExpressionCalls(control, statement.Cond)
	beforeLoop := resolver.flow.snapshot()
	resolver.walkStatements(resolver.table.newScope(control), statement.Body.List)
	if statement.Post != nil {
		resolver.walkStatement(control, statement.Post)
	}
	resolver.flow.restore(beforeLoop)
}

// walkRange walks one iteration of a range loop. Reassigned (non-:=)
// key/value variables get their pre-loop bindings back afterwards.
func (resolver *goMutationResolver) walkRange(scope *goScope, statement *ast.RangeStmt) {
	control := resolver.table.newScope(scope)
	resolver.walkExpressionCalls(control, statement.X)
	beforeLoop := resolver.flow.snapshot()
	rangeBindings := reassignedRangeBindings(control, statement)
	resolver.bindRange(control, statement)
	resolver.walkStatements(resolver.table.newScope(control), statement.Body.List)
	for symbol, binding := range rangeBindings {
		restoreGoSymbolBinding(symbol, binding)
	}
	resolver.flow.restore(beforeLoop)
}

// reassignedRangeBindings snapshots the existing symbols a non-defining range
// statement assigns its key and value to.
func reassignedRangeBindings(control *goScope, statement *ast.RangeStmt) map[*goSymbol]goSymbolBindingState {
	rangeBindings := make(map[*goSymbol]goSymbolBindingState)
	if statement.Tok == token.DEFINE {
		return rangeBindings
	}
	for _, expression := range []ast.Expr{statement.Key, statement.Value} {
		if identifier, ok := expression.(*ast.Ident); ok {
			if symbol := control.lookup(identifier.Name); symbol != nil {
				rangeBindings[symbol] = snapshotGoSymbolBinding(symbol)
			}
		}
	}
	return rangeBindings
}
