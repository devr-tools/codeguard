package performance

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// baseFunctionLoopDepths parses the base-ref revision of a file and returns
// the maximum loop-nesting depth per function key. The base blob is not part
// of the working tree, so it deliberately bypasses the shared parse cache.
func baseFunctionLoopDepths(rel string, data []byte) (map[string]int, error) {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, rel, data, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	depths := make(map[string]int)
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		depth, _ := goMaxLoopNesting(fset, fn.Body)
		depths[goFunctionKey(fn)] = depth
	}
	return depths, nil
}

// goFunctionKey identifies a function across revisions: methods are keyed as
// ReceiverType.Name so same-named methods on different types stay distinct.
func goFunctionKey(fn *ast.FuncDecl) string {
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		if recv := receiverBaseTypeName(fn.Recv.List[0].Type); recv != "" {
			return recv + "." + fn.Name.Name
		}
	}
	return fn.Name.Name
}

func receiverBaseTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return receiverBaseTypeName(t.X)
	case *ast.IndexExpr: // generic receiver: T[P]
		return receiverBaseTypeName(t.X)
	case *ast.IndexListExpr: // generic receiver: T[P1, P2]
		return receiverBaseTypeName(t.X)
	}
	return ""
}

// goMaxLoopNesting returns the maximum syntactic loop-nesting depth in a
// function body and the line of the first loop reaching that depth. Loops
// inside function literals count at their syntactic depth: a closure launched
// per iteration that itself loops still multiplies the iteration space.
func goMaxLoopNesting(fset *token.FileSet, body *ast.BlockStmt) (int, int) {
	maxDepth := 0
	deepestLine := 0
	record := func(depth int, pos token.Pos) {
		if depth > maxDepth {
			maxDepth = depth
			deepestLine = fset.Position(pos).Line
		}
	}
	var walk func(node ast.Node, depth int)
	walk = func(node ast.Node, depth int) {
		ast.Inspect(node, func(child ast.Node) bool {
			loopBody := goLoopBody(child)
			if loopBody == nil {
				return true
			}
			record(depth+1, child.Pos())
			walk(loopBody, depth+1)
			return false
		})
	}
	walk(body, 0)
	return maxDepth, deepestLine
}

func changedRangesIntersect(changed core.ChangedLineRanges, start int, end int) bool {
	if changed.AllChanged {
		return true
	}
	for _, r := range changed.Ranges {
		if r[0] <= end && r[1] >= start {
			return true
		}
	}
	return false
}

// firstChangedLineInSpan returns the smallest changed line within
// [start, end], or start when the whole file counts as changed.
func firstChangedLineInSpan(changed core.ChangedLineRanges, start int, end int) int {
	if changed.AllChanged {
		return start
	}
	best := 0
	for _, r := range changed.Ranges {
		if r[0] > end || r[1] < start {
			continue
		}
		lo := r[0]
		if lo < start {
			lo = start
		}
		if best == 0 || lo < best {
			best = lo
		}
	}
	if best == 0 {
		return start
	}
	return best
}

func sortedChangedPaths(scope map[string]core.ChangedLineRanges) []string {
	paths := make([]string, 0, len(scope))
	for path := range scope {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}
