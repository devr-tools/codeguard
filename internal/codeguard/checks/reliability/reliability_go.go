package reliability

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func goFindingsForFile(env support.Context, file string, data []byte) []core.Finding {
	fset, parsed, err := support.ParseGoSource(env, file, data)
	if err != nil {
		return nil
	}
	rules := env.Config.Checks.ReliabilityRules
	findings := make([]core.Finding, 0)
	httpAliases := importAliases(parsed, "net/http")
	hasShutdown := fileHasSelector(parsed, "Shutdown") || fileHasSelector(parsed, "NotifyContext") || fileHasSelector(parsed, "Notify")

	ast.Inspect(parsed, func(node ast.Node) bool {
		if n, ok := node.(*ast.FuncDecl); ok {
			findings = append(findings, functionReliabilityFindings(env, file, fset, n, rules, httpAliases, hasShutdown)...)
			return false
		}
		return true
	})
	findings = append(findings, partialFailureHiddenFindings(env, file, data)...)

	return support.DedupeFindings(findings, func(finding core.Finding) string {
		return finding.RuleID + "|" + finding.Path + "|" + fmt.Sprintf("%d", finding.Line) + "|" + finding.Message
	})
}

func functionReliabilityFindings(env support.Context, file string, fset *token.FileSet, fn *ast.FuncDecl, rules core.ReliabilityRulesConfig, httpAliases map[string]struct{}, hasShutdown bool) []core.Finding {
	if fn.Body == nil {
		return nil
	}
	findings := make([]core.Finding, 0)
	hasContextParam := funcHasContextParam(fn)
	deferCloseVars := deferredCloseVars(fn.Body)
	boundedClients, boundedRequests := boundedHTTPValues(fn.Body, httpAliases)
	goroutines := 0
	loopDepth := 0
	var loopStack []bool

	ast.Inspect(fn.Body, func(node ast.Node) bool {
		if node == nil {
			if loopStack[len(loopStack)-1] {
				loopDepth--
			}
			loopStack = loopStack[:len(loopStack)-1]
			return true
		}
		switch n := node.(type) {
		case *ast.GoStmt:
			goroutines++
			if enabled(rules.DetectUnboundedWork) && loopDepth > 0 {
				pos := fset.Position(n.Go)
				findings = append(findings, newFinding(env, "reliability.unbounded-work", "warn", file, pos.Line, pos.Column, "goroutine launched inside a loop without an obvious work bound", "high", "work", "goroutine-in-loop"))
			}
		case *ast.ForStmt:
			findings = append(findings, retryFindings(env, file, fset, n, rules)...)
		case *ast.RangeStmt:
			if enabled(rules.DetectRetryWithoutBackoff) && loopLooksLikeRetry(n.Body) && !blockHasBackoff(n.Body) {
				pos := fset.Position(n.For)
				findings = append(findings, newFinding(env, "reliability.retry-without-backoff", "warn", file, pos.Line, pos.Column, "retry-like loop has no visible backoff or jitter", "medium", "retry", "range-loop"))
			}
		case *ast.CallExpr:
			findings = append(findings, callReliabilityFindings(env, file, fset, n, rules, httpAliases, boundedClients, boundedRequests, hasContextParam, hasShutdown)...)
		case *ast.AssignStmt:
			findings = append(findings, assignmentReliabilityFindings(env, file, fset, n, rules, httpAliases, deferCloseVars)...)
		case *ast.ReturnStmt:
			if enabled(rules.DetectLostErrorContext) {
				findings = append(findings, lostErrorContextFindings(env, file, fset, n)...)
			}
		}
		_, isFor := node.(*ast.ForStmt)
		_, isRange := node.(*ast.RangeStmt)
		isLoop := isFor || isRange
		loopStack = append(loopStack, isLoop)
		if isLoop {
			loopDepth++
		}
		return true
	})

	if enabled(rules.DetectMissingConcurrencyLimit) && rules.MaxInlineGoroutinesPerFunction > 0 && goroutines > rules.MaxInlineGoroutinesPerFunction {
		pos := fset.Position(fn.Pos())
		findings = append(findings, newFinding(env, "reliability.missing-concurrency-limit", "warn", file, pos.Line, pos.Column, fmt.Sprintf("function launches %d goroutines without an obvious concurrency limit", goroutines), "medium", "goroutines", fmt.Sprintf("%d", goroutines)))
	}

	return findings
}

func callReliabilityFindings(env support.Context, file string, fset *token.FileSet, call *ast.CallExpr, rules core.ReliabilityRulesConfig, httpAliases map[string]struct{}, boundedClients map[string]struct{}, boundedRequests map[string]struct{}, hasContextParam bool, hasShutdown bool) []core.Finding {
	findings := make([]core.Finding, 0, 2)
	pos := fset.Position(call.Pos())
	if enabled(rules.DetectMissingTimeout) && isUnboundedHTTPCall(call, httpAliases, boundedClients, boundedRequests) {
		findings = append(findings, newFinding(env, "reliability.missing-timeout", "fail", file, pos.Line, pos.Column, "outbound HTTP call is made without a request context or client timeout", "high", "call", callName(call)))
	}
	if enabled(rules.DetectMissingCancellation) && hasContextParam && isBackgroundContextCall(call) {
		findings = append(findings, newFinding(env, "reliability.missing-cancellation", "warn", file, pos.Line, pos.Column, "function with caller context creates detached background context for downstream work", "high", "context", callName(call)))
	}
	if enabled(rules.DetectRecoverablePanic) && isPanicCall(call) {
		findings = append(findings, newFinding(env, "reliability.recoverable-panic", "fail", file, pos.Line, pos.Column, "production code uses panic for a recoverable failure path", "medium", "call", "panic"))
	}
	if enabled(rules.DetectMissingGracefulShutdown) && !hasShutdown && isListenAndServeCall(call, httpAliases) {
		findings = append(findings, newFinding(env, "reliability.missing-graceful-shutdown", "warn", file, pos.Line, pos.Column, "server starts without visible signal handling or graceful shutdown", "medium", "server", callName(call)))
	}
	return findings
}

func assignmentReliabilityFindings(env support.Context, file string, fset *token.FileSet, assign *ast.AssignStmt, rules core.ReliabilityRulesConfig, httpAliases map[string]struct{}, deferCloseVars map[string]struct{}) []core.Finding {
	findings := make([]core.Finding, 0, 2)
	if enabled(rules.DetectSwallowedError) && assignmentSwallowsCallError(assign) {
		pos := fset.Position(assign.Pos())
		findings = append(findings, newFinding(env, "reliability.swallowed-error", "fail", file, pos.Line, pos.Column, "call result error is assigned to the blank identifier", "high", "assignment", "blank-error"))
	}
	if enabled(rules.DetectResourceLeak) {
		for _, name := range assignedHTTPResponseVars(assign, httpAliases) {
			if _, closed := deferCloseVars[name]; !closed {
				pos := fset.Position(assign.Pos())
				findings = append(findings, newFinding(env, "reliability.resource-leak", "fail", file, pos.Line, pos.Column, "HTTP response body is not closed on the acquisition path", "high", "resource", "http-response-body"))
			}
		}
	}
	return findings
}

func retryFindings(env support.Context, file string, fset *token.FileSet, loop *ast.ForStmt, rules core.ReliabilityRulesConfig) []core.Finding {
	if !loopLooksLikeRetry(loop.Body) && loop.Cond != nil {
		return nil
	}
	findings := make([]core.Finding, 0, 3)
	pos := fset.Position(loop.For)
	if enabled(rules.DetectUnboundedRetry) && loop.Cond == nil {
		findings = append(findings, newFinding(env, "reliability.unbounded-retry", "fail", file, pos.Line, pos.Column, "retry-like loop has no condition limiting attempts", "high", "retry", "unbounded-for"))
	}
	if enabled(rules.DetectRetryWithoutBackoff) && !blockHasBackoff(loop.Body) {
		findings = append(findings, newFinding(env, "reliability.retry-without-backoff", "warn", file, pos.Line, pos.Column, "retry-like loop has no visible backoff, sleep, ticker, or jitter", "medium", "retry", "no-backoff"))
	}
	if enabled(rules.DetectNonIdempotentRetry) && blockHasNonIdempotentCall(loop.Body) && !blockHasIdempotencyEvidence(loop.Body) {
		findings = append(findings, newFinding(env, "reliability.non-idempotent-retry", "fail", file, pos.Line, pos.Column, "retry-like loop wraps a non-idempotent side effect without idempotency evidence", "medium", "retry", "side-effect"))
	}
	return findings
}

func lostErrorContextFindings(env support.Context, file string, fset *token.FileSet, ret *ast.ReturnStmt) []core.Finding {
	for _, result := range ret.Results {
		call, ok := result.(*ast.CallExpr)
		if !ok || !isErrorsNewCall(call) {
			continue
		}
		pos := fset.Position(call.Pos())
		return []core.Finding{newFinding(env, "reliability.lost-error-context", "warn", file, pos.Line, pos.Column, "error is replaced with a new generic error instead of wrapping the original cause", "medium", "error", "errors.New")}
	}
	return nil
}

func newFinding(env support.Context, ruleID string, level string, path string, line int, column int, message string, confidence string, metaKey string, metaValue string) core.Finding {
	return env.NewFinding(support.FindingInput{
		RuleID:     ruleID,
		Level:      level,
		Path:       path,
		Line:       line,
		Column:     column,
		Message:    message,
		Confidence: confidence,
		Metadata:   map[string]string{metaKey: metaValue},
	})
}
