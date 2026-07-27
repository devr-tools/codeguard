package data

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func goFindingsForFile(env support.Context, file string, data []byte) []core.Finding {
	fset, parsed, err := support.ParseGoSource(env, file, data)
	if err != nil {
		return nil
	}
	rules := env.Config.Checks.DataRules
	findings := make([]core.Finding, 0)
	findings = append(findings, sqlTextFindings(env, file, fset, parsed, rules)...)
	findings = append(findings, commentFindings(env, file, fset, parsed, rules)...)

	ast.Inspect(parsed, func(node ast.Node) bool {
		fn, ok := node.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			return true
		}
		findings = append(findings, functionDataFindings(env, file, fset, fn, rules)...)
		return false
	})

	return support.DedupeFindings(findings, func(finding core.Finding) string {
		return finding.RuleID + "|" + finding.Path + "|" + fmt.Sprintf("%d", finding.Line) + "|" + finding.Message
	})
}

func functionDataFindings(env support.Context, file string, fset *token.FileSet, fn *ast.FuncDecl, rules core.DataRulesConfig) []core.Finding {
	summary := summarizeFunction(fn.Body)
	findings := make([]core.Finding, 0)
	pos := fset.Position(fn.Pos())
	inTransaction := summary.transactionCalls > 0

	if enabled(rules.DetectReadModifyWriteRace) && summary.readCalls > 0 && summary.writeCalls > 0 && !inTransaction {
		findings = append(findings, newFinding(env, "data.read-modify-write-race", "fail", file, pos.Line, pos.Column, "function reads state and writes derived state without a transaction or atomic update boundary", "medium", "pattern", "read-modify-write"))
	}
	if enabled(rules.DetectMissingTransaction) && rules.MaxWritesWithoutTransaction >= 0 && summary.writeCalls > rules.MaxWritesWithoutTransaction && !inTransaction {
		findings = append(findings, newFinding(env, "data.missing-transaction-boundary", "fail", file, pos.Line, pos.Column, fmt.Sprintf("function performs %d persistence writes without an obvious transaction boundary", summary.writeCalls), "medium", "writes", fmt.Sprintf("%d", summary.writeCalls)))
	}
	if enabled(rules.DetectUnsafeDualWrite) && summary.writeCalls > 0 && summary.sideEffectCalls > 0 && !summary.hasOutbox {
		findings = append(findings, newFinding(env, "data.unsafe-dual-write", "fail", file, pos.Line, pos.Column, "function writes state and performs an external side effect without an obvious consistency strategy", "medium", "pattern", "write-plus-side-effect"))
	}
	if enabled(rules.DetectMissingOutboxStrategy) && summary.writeCalls > 0 && summary.publishCalls > 0 && !summary.hasOutbox {
		findings = append(findings, newFinding(env, "data.missing-outbox-strategy", "fail", file, pos.Line, pos.Column, "state write is paired with event publishing without outbox evidence", "medium", "pattern", "write-plus-publish"))
	}
	if enabled(rules.DetectNonIdempotentConsumer) && looksLikeConsumer(fn) && summary.sideEffectCalls > 0 && !summary.hasIdempotency {
		findings = append(findings, newFinding(env, "data.non-idempotent-consumer", "fail", file, pos.Line, pos.Column, "message or event handler performs side effects without idempotency evidence", "medium", "consumer", fn.Name.Name))
	}
	if enabled(rules.DetectMissingDeduplication) && looksLikeConsumer(fn) && !summary.hasIdempotency {
		findings = append(findings, newFinding(env, "data.missing-deduplication", "warn", file, pos.Line, pos.Column, "message or event handler has no visible deduplication guard", "medium", "consumer", fn.Name.Name))
	}
	if enabled(rules.DetectCacheWithoutPolicy) && summary.cacheSetWithoutTTL > 0 {
		findings = append(findings, newFinding(env, "data.cache-without-policy", "warn", file, pos.Line, pos.Column, "cache writes lack visible TTL, invalidation, or ownership policy", "medium", "cache_sets", fmt.Sprintf("%d", summary.cacheSetWithoutTTL)))
	}
	if enabled(rules.DetectSideEffectInTransaction) {
		findings = append(findings, sideEffectsInTransactions(env, file, fset, fn.Body)...)
	}
	return findings
}

type functionSummary struct {
	readCalls          int
	writeCalls         int
	publishCalls       int
	sideEffectCalls    int
	transactionCalls   int
	cacheSetWithoutTTL int
	hasOutbox          bool
	hasIdempotency     bool
}

func summarizeFunction(block *ast.BlockStmt) functionSummary {
	var summary functionSummary
	ast.Inspect(block, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		name := strings.ToLower(callName(call))
		switch {
		case containsAny(name, "transaction", "withtx", "intx", "begintx", ".tx"):
			summary.transactionCalls++
		case containsAny(name, "get", "select", "query", "find", "load", "read"):
			summary.readCalls++
		}
		if containsAny(name, "create", "update", "delete", "save", "insert", "upsert", "exec") {
			summary.writeCalls++
		}
		if containsAny(name, "publish", "emit", "enqueue") {
			summary.publishCalls++
		}
		if containsAny(name, "publish", "emit", "enqueue", "send", "post", "put", "patch", "delete", "charge", "email", "webhook") {
			summary.sideEffectCalls++
		}
		if containsAny(name, "outbox") {
			summary.hasOutbox = true
		}
		if containsAny(name, "idempot", "dedupe", "dedup", "processed", "messageid", "eventid") {
			summary.hasIdempotency = true
		}
		if isCacheSetWithoutTTL(call) {
			summary.cacheSetWithoutTTL++
		}
		return true
	})
	return summary
}

func sideEffectsInTransactions(env support.Context, file string, fset *token.FileSet, block *ast.BlockStmt) []core.Finding {
	findings := make([]core.Finding, 0)
	ast.Inspect(block, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || !looksLikeTransactionCall(call) {
			return true
		}
		for _, arg := range call.Args {
			fn, ok := arg.(*ast.FuncLit)
			if !ok || fn.Body == nil {
				continue
			}
			if blockHasExternalSideEffect(fn.Body) {
				pos := fset.Position(call.Pos())
				findings = append(findings, newFinding(env, "data.side-effect-in-transaction", "fail", file, pos.Line, pos.Column, "transaction callback performs an external side effect that may not roll back safely", "high", "transaction", callName(call)))
			}
		}
		return true
	})
	return findings
}

func sqlTextFindings(env support.Context, file string, fset *token.FileSet, parsed *ast.File, rules core.DataRulesConfig) []core.Finding {
	findings := make([]core.Finding, 0)
	ast.Inspect(parsed, func(node ast.Node) bool {
		lit, ok := node.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		text := strings.ToLower(strings.Trim(lit.Value, "`\""))
		if enabled(rules.DetectUnstablePagination) && strings.Contains(text, " limit ") && strings.Contains(text, " offset ") && !strings.Contains(text, " order by ") {
			pos := fset.Position(lit.Pos())
			findings = append(findings, newFinding(env, "data.unstable-pagination", "warn", file, pos.Line, pos.Column, "SQL pagination uses LIMIT/OFFSET without deterministic ORDER BY", "high", "query", "limit-offset"))
		}
		if enabled(rules.DetectUnboundedRead) && strings.Contains(text, "select ") && !strings.Contains(text, " limit ") && !strings.Contains(text, " where ") {
			pos := fset.Position(lit.Pos())
			findings = append(findings, newFinding(env, "data.unbounded-read", "warn", file, pos.Line, pos.Column, "SQL read has no visible WHERE, LIMIT, cursor, or streaming bound", "medium", "query", "select-without-bound"))
		}
		return true
	})
	return findings
}

func commentFindings(env support.Context, file string, fset *token.FileSet, parsed *ast.File, rules core.DataRulesConfig) []core.Finding {
	if !enabled(rules.DetectExactlyOnceAssumption) {
		return nil
	}
	findings := make([]core.Finding, 0)
	for _, group := range parsed.Comments {
		text := strings.ToLower(group.Text())
		if strings.Contains(text, "exactly once") && !containsAny(text, "idempot", "dedupe", "dedup") {
			pos := fset.Position(group.Pos())
			findings = append(findings, newFinding(env, "data.exactly-once-assumption", "warn", file, pos.Line, pos.Column, "comment assumes exactly-once delivery without idempotency or deduplication evidence", "low", "comment", "exactly-once"))
		}
	}
	return findings
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

func looksLikeConsumer(fn *ast.FuncDecl) bool {
	name := strings.ToLower(fn.Name.Name)
	if containsAny(name, "handle", "consume", "process", "onevent", "onmessage") {
		return true
	}
	if fn.Type == nil || fn.Type.Params == nil {
		return false
	}
	for _, field := range fn.Type.Params.List {
		if containsAny(strings.ToLower(exprString(field.Type)), "event", "message", "msg") {
			return true
		}
	}
	return false
}

func looksLikeTransactionCall(call *ast.CallExpr) bool {
	name := strings.ToLower(callName(call))
	return containsAny(name, "transaction", "withtx", "intx", "begintx")
}

func blockHasExternalSideEffect(block *ast.BlockStmt) bool {
	found := false
	ast.Inspect(block, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if containsAny(strings.ToLower(callName(call)), "publish", "emit", "send", "post", "charge", "email", "webhook") {
			found = true
			return false
		}
		return true
	})
	return found
}

func isCacheSetWithoutTTL(call *ast.CallExpr) bool {
	name := strings.ToLower(callName(call))
	if !strings.Contains(name, "cache") || !strings.HasSuffix(name, ".set") {
		return false
	}
	if len(call.Args) >= 3 {
		return false
	}
	return !containsAny(name, "ttl", "expire")
}

func containsAny(value string, tokens ...string) bool {
	for _, token := range tokens {
		if strings.Contains(value, token) {
			return true
		}
	}
	return false
}

func callName(call *ast.CallExpr) string {
	return exprString(call.Fun)
}

func exprString(expr ast.Expr) string {
	switch n := expr.(type) {
	case *ast.Ident:
		return n.Name
	case *ast.SelectorExpr:
		left := exprString(n.X)
		if left == "" {
			return n.Sel.Name
		}
		return left + "." + n.Sel.Name
	case *ast.StarExpr:
		return "*" + exprString(n.X)
	case *ast.CallExpr:
		return exprString(n.Fun)
	default:
		return fmt.Sprintf("%T", expr)
	}
}
