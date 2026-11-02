package support

import (
	"go/ast"
	"go/token"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// CyclomaticComplexityFor returns the Go complexity function for a
// quality_rules.cyclomatic_counting mode: gocyclo selects
// CyclomaticComplexity, and anything else (including the unset 1.x default)
// selects LegacyCyclomaticComplexity. Config validation rejects unknown modes
// before a scan, so the fallback only matters for configs built in code.
func CyclomaticComplexityFor(mode string) func(body *ast.BlockStmt) int {
	if core.NormalizeCyclomaticCounting(mode) == core.CyclomaticCountingGocyclo {
		return CyclomaticComplexity
	}
	return LegacyCyclomaticComplexity
}

// CyclomaticComplexity returns the McCabe cyclomatic complexity of a Go
// function body, counted the way gocyclo counts it: 1, plus one for every
// if, for, and range statement, every non-default case of a switch, type
// switch, or select, and every && and || operator (each short-circuit
// operator is its own decision point). Function literals inside the body
// count toward the enclosing function. This is the
// quality_rules.cyclomatic_counting: gocyclo mode.
func CyclomaticComplexity(body *ast.BlockStmt) int {
	return countDecisionPoints(body, isDecisionPoint)
}

// LegacyCyclomaticComplexity returns the Go complexity codeguard reported
// through v1.10.1: 1, plus one for every if, for, and range statement and
// every case or comm clause, default clauses included; && and || are not
// counted. Function literals count toward the enclosing function. This is
// the quality_rules.cyclomatic_counting: legacy mode, the 1.x default.
func LegacyCyclomaticComplexity(body *ast.BlockStmt) int {
	return countDecisionPoints(body, isLegacyDecisionPoint)
}

func countDecisionPoints(body *ast.BlockStmt, decision func(ast.Node) bool) int {
	if body == nil {
		return 0
	}
	complexity := 1
	ast.Inspect(body, func(n ast.Node) bool {
		if decision(n) {
			complexity++
		}
		return true
	})
	return complexity
}

func isDecisionPoint(n ast.Node) bool {
	switch node := n.(type) {
	case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt:
		return true
	case *ast.CaseClause:
		return node.List != nil // default: is not a decision point
	case *ast.CommClause:
		return node.Comm != nil // default: is not a decision point
	case *ast.BinaryExpr:
		return node.Op == token.LAND || node.Op == token.LOR
	}
	return false
}

func isLegacyDecisionPoint(n ast.Node) bool {
	switch n.(type) {
	case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.CaseClause, *ast.CommClause:
		return true
	}
	return false
}
