package performance

import "go/ast"

func walkASTWithStack(root ast.Node, visit func(node ast.Node, stack []ast.Node) bool) {
	stack := make([]ast.Node, 0, 32)
	ast.Inspect(root, func(node ast.Node) bool {
		if node == nil {
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			return false
		}

		parentStack := stack
		stack = append(stack, node)
		return visit(node, parentStack)
	})
}
