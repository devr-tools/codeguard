package quality

import (
	"go/ast"
	"go/token"
)

func (resolver *goMutationResolver) walkExclusiveBranches(scope *goScope, statements []ast.Stmt, exhaustive bool) {
	before := resolver.flow.snapshot()
	states := make([]goBranchState, 0, len(statements)+1)
	for _, statement := range statements {
		resolver.flow.restore(before)
		resolver.walkStatement(scope, statement)
		states = append(states, resolver.flow.snapshot())
	}
	if !exhaustive || len(states) == 0 {
		states = append(states, before)
	}
	resolver.flow.restore(resolver.flow.merge(before, states...))
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
	before := resolver.flow.snapshot()
	states := make([]goBranchState, 0, len(statements)+1)
	for entry := range statements {
		resolver.flow.restore(before)
		resolver.walkStatement(scope, statements[entry])
		for current := entry; goCaseFallsThrough(statements[current]) && current+1 < len(statements); current++ {
			next, ok := statements[current+1].(*ast.CaseClause)
			if !ok {
				break
			}
			resolver.walkStatements(resolver.table.newScope(scope), next.Body)
		}
		states = append(states, resolver.flow.snapshot())
	}
	if !exhaustive || len(states) == 0 {
		states = append(states, before)
	}
	resolver.flow.restore(resolver.flow.merge(before, states...))
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

func (resolver *goMutationResolver) walkSwitch(scope *goScope, statement *ast.SwitchStmt) {
	control := resolver.table.newScope(scope)
	if statement.Init != nil {
		resolver.walkStatement(control, statement.Init)
	}
	resolver.walkExpressionCalls(control, statement.Tag)
	resolver.walkSwitchBranches(control, statement.Body.List, goBranchesHaveDefault(statement.Body.List))
}

func (resolver *goMutationResolver) walkTypeSwitch(scope *goScope, statement *ast.TypeSwitchStmt) {
	control := resolver.table.newScope(scope)
	if statement.Init != nil {
		resolver.walkStatement(control, statement.Init)
	}
	if statement.Assign != nil {
		resolver.walkStatement(control, statement.Assign)
	}
	resolver.walkExclusiveBranches(control, statement.Body.List, goBranchesHaveDefault(statement.Body.List))
}

func (resolver *goMutationResolver) walkCaseClause(scope *goScope, clause *ast.CaseClause) {
	child := resolver.table.newScope(scope)
	for _, expression := range clause.List {
		resolver.walkExpressionCalls(child, expression)
	}
	resolver.walkStatements(child, clause.Body)
}

func (resolver *goMutationResolver) walkCommClause(scope *goScope, clause *ast.CommClause) {
	child := resolver.table.newScope(scope)
	if clause.Comm != nil {
		resolver.walkStatement(child, clause.Comm)
	}
	resolver.walkStatements(child, clause.Body)
}
