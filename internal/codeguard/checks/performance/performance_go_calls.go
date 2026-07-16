package performance

import (
	"go/ast"
	"go/token"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func goLoopCallFindings(env support.Context, file string, fset *token.FileSet, parsed *ast.File) []core.Finding {
	cfg := newGoLoopCallConfig(env, file, parsed)
	if !cfg.enabled() {
		return nil
	}

	findings := make([]core.Finding, 0)
	warn := func(ruleID string, pos token.Position, message string) {
		findings = append(findings, warnFinding(env, ruleID, file, pos.Line, pos.Column, message))
	}

	walkASTWithStack(parsed, func(node ast.Node, stack []ast.Node) bool {
		switch node := node.(type) {
		case *ast.DeferStmt:
			if cfg.detectDefer && hasLoopAncestorWithinFunc(stack) {
				warn("performance.go.defer-in-loop", fset.Position(node.Defer),
					"defer inside a loop runs only at function exit, so deferred resources accumulate each iteration; release explicitly or extract the loop body into a function")
			}
		case *ast.CallExpr:
			if finding := cfg.callFinding(stack, node, fset); finding != nil {
				warn(finding.ruleID, finding.pos, finding.message)
			}
		}
		return true
	})
	return findings
}

type goLoopCallConfig struct {
	file         string
	detectRegex  bool
	detectDefer  bool
	detectSleep  bool
	detectTimer  bool
	detectReads  bool
	regexAliases map[string]struct{}
	timeAliases  map[string]struct{}
	readAliases  map[string]struct{}
	httpAliases  map[string]struct{}
}

type goLoopCallFinding struct {
	ruleID  string
	pos     token.Position
	message string
}

func newGoLoopCallConfig(env support.Context, file string, parsed *ast.File) goLoopCallConfig {
	rules := env.Config.Checks.PerformanceRules
	readAliases := importAliasesForPath(parsed, "io")
	for alias := range importAliasesForPath(parsed, "io/ioutil") {
		readAliases[alias] = struct{}{}
	}
	return goLoopCallConfig{
		file:         file,
		detectRegex:  toggleEnabled(rules.DetectRegexCompileInLoop),
		detectDefer:  toggleEnabled(rules.DetectDeferInLoop),
		detectSleep:  toggleEnabled(rules.DetectSleepInLoop),
		detectTimer:  toggleEnabled(rules.DetectTimerLeaks),
		detectReads:  toggleEnabled(rules.DetectUnboundedReads),
		regexAliases: importAliasesForPath(parsed, "regexp"),
		timeAliases:  importAliasesForPath(parsed, "time"),
		readAliases:  readAliases,
		httpAliases:  importAliasesForPath(parsed, "net/http"),
	}
}

func (c goLoopCallConfig) enabled() bool {
	return c.detectRegex || c.detectDefer || c.detectSleep || c.detectTimer || c.detectReads
}

func (c goLoopCallConfig) callFinding(stack []ast.Node, call *ast.CallExpr, fset *token.FileSet) *goLoopCallFinding {
	alias, name, ok := packageCall(call)
	if !ok {
		return nil
	}
	inLoop := hasLoopAncestor(stack)
	pos := fset.Position(call.Pos())
	switch {
	case inLoop && c.detectRegex && aliasHas(c.regexAliases, alias) && nameIn(regexCompileNames, name) && literalPatternArg(call):
		return &goLoopCallFinding{ruleID: "performance.regex-compile-in-loop", pos: pos, message: "regular expression compiled inside a loop; compile it once before the loop or as a package-level variable"}
	case inLoop && c.detectSleep && !strings.HasSuffix(c.file, "_test.go") && aliasHas(c.timeAliases, alias) && name == "Sleep":
		return &goLoopCallFinding{ruleID: "performance.go.sleep-in-loop", pos: pos, message: "time.Sleep inside a loop usually marks polling; prefer a time.Ticker, a channel signal, or a backoff helper"}
	case inLoop && c.detectTimer && aliasHas(c.timeAliases, alias) && name == "After":
		return &goLoopCallFinding{ruleID: "performance.go.timer-leak-in-loop", pos: pos, message: "time.After inside a loop allocates a timer every iteration that is not collected until it fires; reuse a time.NewTimer or time.NewTicker"}
	case c.detectReads && aliasHas(c.readAliases, alias) && name == "ReadAll" && !readerIsLimited(call) && c.shouldWarnReadAll(stack):
		return &goLoopCallFinding{ruleID: "performance.unbounded-read", pos: pos, message: "ReadAll loads the entire input into memory; bound it with io.LimitReader or process the stream incrementally"}
	default:
		return nil
	}
}

func (c goLoopCallConfig) shouldWarnReadAll(stack []ast.Node) bool {
	if hasLoopAncestor(stack) {
		return true
	}
	fn := enclosingFunc(stack)
	return fn != nil && isLikelyHTTPHandler(fn, c.httpAliases)
}

func literalPatternArg(call *ast.CallExpr) bool {
	if len(call.Args) == 0 {
		return false
	}
	lit, ok := call.Args[0].(*ast.BasicLit)
	return ok && lit.Kind == token.STRING
}
