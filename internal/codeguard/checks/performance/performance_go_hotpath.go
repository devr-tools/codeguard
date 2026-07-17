package performance

import (
	"go/ast"
	"go/token"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var goSliceMembershipNames = map[string]struct{}{
	"Contains":     {},
	"ContainsFunc": {},
	"Index":        {},
	"IndexFunc":    {},
}

type goHotPathConfig struct {
	slicesAliases map[string]struct{}
}

type goHotPathFinding struct {
	ruleID  string
	pos     token.Position
	message string
}

func goHotPathFindings(env support.Context, file string, fset *token.FileSet, parsed *ast.File) []core.Finding {
	if !toggleEnabled(env.Config.Checks.PerformanceRules.DetectHotPathPatterns) {
		return nil
	}
	cfg := newGoHotPathConfig(parsed)
	if !cfg.enabled() {
		return nil
	}

	findings := make([]core.Finding, 0)
	seen := make(map[int]struct{})
	walkASTWithStack(parsed, func(node ast.Node, stack []ast.Node) bool {
		var finding *goHotPathFinding
		switch node := node.(type) {
		case *ast.CallExpr:
			if hasLoopAncestor(stack) {
				finding = cfg.callFinding(node, fset)
			}
		case *ast.BinaryExpr:
			finding = cfg.binaryFinding(node, stack, fset)
		}
		if finding == nil {
			return true
		}
		if _, dup := seen[finding.pos.Line]; !dup {
			seen[finding.pos.Line] = struct{}{}
			findings = append(findings, warnFinding(env, finding.ruleID, file, finding.pos.Line, finding.pos.Column, finding.message))
		}
		return true
	})
	return findings
}

func newGoHotPathConfig(parsed *ast.File) goHotPathConfig {
	aliases := importAliasesForPath(parsed, "slices")
	for alias := range importAliasesForPath(parsed, "golang.org/x/exp/slices") {
		aliases[alias] = struct{}{}
	}
	return goHotPathConfig{slicesAliases: aliases}
}

func (c goHotPathConfig) enabled() bool {
	return true
}

func (c goHotPathConfig) callFinding(call *ast.CallExpr, fset *token.FileSet) *goHotPathFinding {
	alias, name, ok := packageCall(call)
	if !ok || !aliasHas(c.slicesAliases, alias) || !nameIn(goSliceMembershipNames, name) {
		return nil
	}
	pos := fset.Position(call.Pos())
	return &goHotPathFinding{
		ruleID:  "performance.go.slice-membership-in-loop",
		pos:     pos,
		message: "slices membership scan inside a loop is linear each iteration; precompute a map/set or otherwise hoist the lookup if this path is hot",
	}
}

func (c goHotPathConfig) binaryFinding(expr *ast.BinaryExpr, stack []ast.Node, fset *token.FileSet) *goHotPathFinding {
	if expr.Op != token.EQL {
		return nil
	}
	outerVars, innerVars := nestedLoopVarSets(stack)
	if len(outerVars) == 0 || len(innerVars) == 0 {
		return nil
	}
	left := identName(expr.X)
	right := identName(expr.Y)
	if left == "" || right == "" {
		return nil
	}
	if (!outerVars[left] || !innerVars[right]) && (!outerVars[right] || !innerVars[left]) {
		return nil
	}
	pos := fset.Position(expr.Pos())
	return &goHotPathFinding{
		ruleID:  "performance.go.nested-loop-scan",
		pos:     pos,
		message: "nested loops compare outer items against inner items linearly; if this is a membership test on hot data, precompute a map/set for the inner collection",
	}
}

func nestedLoopVarSets(stack []ast.Node) (map[string]bool, map[string]bool) {
	rangeLoops := make([]*ast.RangeStmt, 0, 4)
	for _, node := range stack {
		if loop, ok := node.(*ast.RangeStmt); ok {
			rangeLoops = append(rangeLoops, loop)
		}
	}
	if len(rangeLoops) < 2 {
		return nil, nil
	}
	inner := rangeLoopVarSet(rangeLoops[len(rangeLoops)-1])
	outer := make(map[string]bool)
	for _, loop := range rangeLoops[:len(rangeLoops)-1] {
		for name := range rangeLoopVarSet(loop) {
			outer[name] = true
		}
	}
	return outer, inner
}

func rangeLoopVarSet(loop *ast.RangeStmt) map[string]bool {
	names := make(map[string]bool)
	if loop == nil {
		return names
	}
	if name := identName(loop.Key); name != "" && name != "_" {
		names[name] = true
	}
	if name := identName(loop.Value); name != "" && name != "_" {
		names[name] = true
	}
	return names
}

func identName(expr ast.Expr) string {
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return ""
	}
	return strings.TrimSpace(ident.Name)
}
