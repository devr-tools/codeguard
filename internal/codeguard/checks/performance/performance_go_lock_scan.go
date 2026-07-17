package performance

import "go/ast"

func scanStructuredStmt(scan *goLockHeldScan, stmt ast.Stmt, held map[string]struct{}) bool {
	switch node := stmt.(type) {
	case *ast.BlockStmt:
		scan.scanBlock(node, cloneHeldLocks(held))
	case *ast.IfStmt:
		scanIfStmt(scan, node, held)
	case *ast.ForStmt:
		scanForStmt(scan, node, held)
	case *ast.RangeStmt:
		scanRangeStmt(scan, node, held)
	case *ast.SwitchStmt:
		scanSwitchStmt(scan, node, held)
	case *ast.TypeSwitchStmt:
		scanTypeSwitchStmt(scan, node, held)
	case *ast.SelectStmt:
		scan.scanBlock(node.Body, cloneHeldLocks(held))
	default:
		return false
	}
	return true
}

func scanIfStmt(scan *goLockHeldScan, node *ast.IfStmt, held map[string]struct{}) {
	maybeReportBlockingCall(scan, node.Init, held)
	maybeReportBlockingExpr(scan, node.Cond, held)
	scan.scanBlock(node.Body, cloneHeldLocks(held))
	if node.Else != nil {
		scan.scanStmt(node.Else, cloneHeldLocks(held))
	}
}

func scanForStmt(scan *goLockHeldScan, node *ast.ForStmt, held map[string]struct{}) {
	maybeReportBlockingCall(scan, node.Init, held)
	maybeReportBlockingExpr(scan, node.Cond, held)
	maybeReportBlockingCall(scan, node.Post, held)
	scan.scanBlock(node.Body, cloneHeldLocks(held))
}

func scanRangeStmt(scan *goLockHeldScan, node *ast.RangeStmt, held map[string]struct{}) {
	maybeReportBlockingExpr(scan, node.X, held)
	scan.scanBlock(node.Body, cloneHeldLocks(held))
}

func scanSwitchStmt(scan *goLockHeldScan, node *ast.SwitchStmt, held map[string]struct{}) {
	maybeReportBlockingCall(scan, node.Init, held)
	maybeReportBlockingExpr(scan, node.Tag, held)
	scan.scanBlock(node.Body, cloneHeldLocks(held))
}

func scanTypeSwitchStmt(scan *goLockHeldScan, node *ast.TypeSwitchStmt, held map[string]struct{}) {
	maybeReportBlockingCall(scan, node.Init, held)
	maybeReportBlockingCall(scan, node.Assign, held)
	scan.scanBlock(node.Body, cloneHeldLocks(held))
}

func maybeReportBlockingExpr(scan *goLockHeldScan, expr ast.Expr, held map[string]struct{}) {
	if expr == nil {
		return
	}
	reportBlockingNodes(scan, expr, held)
}

func maybeReportBlockingCall(scan *goLockHeldScan, node ast.Node, held map[string]struct{}) {
	if node == nil {
		return
	}
	reportBlockingNodes(scan, node, held)
}

func reportBlockingNodes(scan *goLockHeldScan, root ast.Node, held map[string]struct{}) {
	if len(held) == 0 || root == nil {
		return
	}
	ast.Inspect(root, func(node ast.Node) bool {
		if _, ok := node.(*ast.FuncLit); ok {
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if blockingCallKind(scan.cfg, call) == "" {
			return true
		}
		pos := scan.fset.Position(call.Pos())
		if _, dup := scan.seen[pos.Line]; dup {
			return false
		}
		scan.seen[pos.Line] = struct{}{}
		scan.findings = append(scan.findings, warnFinding(scan.env, "performance.go.lock-held-across-blocking-call", scan.file, pos.Line, pos.Column,
			"mutex held across a blocking call can serialize callers and amplify tail latency; copy the needed state and release the lock before the call"))
		return false
	})
}
