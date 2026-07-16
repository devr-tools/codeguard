package performance

import (
	"go/ast"
	"go/token"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var regexCompileNames = map[string]struct{}{
	"Compile":          {},
	"MustCompile":      {},
	"CompilePOSIX":     {},
	"MustCompilePOSIX": {},
}

// goLoopCallFindings flags loop bodies that repeat work belonging outside the
// loop (regex compilation, sleeps, leaked timers, accumulating defers) plus
// unbounded whole-input reads in loops or HTTP request paths.
func goLoopCallFindings(env support.Context, file string, fset *token.FileSet, parsed *ast.File) []core.Finding {
	rules := env.Config.Checks.PerformanceRules
	detectRegex := toggleEnabled(rules.DetectRegexCompileInLoop)
	detectDefer := toggleEnabled(rules.DetectDeferInLoop)
	detectSleep := toggleEnabled(rules.DetectSleepInLoop)
	detectTimer := toggleEnabled(rules.DetectTimerLeaks)
	detectReads := toggleEnabled(rules.DetectUnboundedReads)
	if !detectRegex && !detectDefer && !detectSleep && !detectTimer && !detectReads {
		return nil
	}

	regexAliases := importAliasesForPath(parsed, "regexp")
	timeAliases := importAliasesForPath(parsed, "time")
	readAliases := importAliasesForPath(parsed, "io")
	for alias := range importAliasesForPath(parsed, "io/ioutil") {
		readAliases[alias] = struct{}{}
	}
	httpAliases := importAliasesForPath(parsed, "net/http")

	findings := make([]core.Finding, 0)
	warn := func(ruleID string, pos token.Position, message string) {
		findings = append(findings, warnFinding(env, ruleID, file, pos.Line, pos.Column, message))
	}

	stack := make([]ast.Node, 0, 32)
	ast.Inspect(parsed, func(n ast.Node) bool {
		if n == nil {
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			return false
		}
		stack = append(stack, n)

		switch node := n.(type) {
		case *ast.DeferStmt:
			// Defer scopes to the enclosing function: a defer inside a func
			// literal launched from a loop runs per goroutine/function exit and
			// does not accumulate, so the ancestor walk stops at the boundary.
			if detectDefer && hasLoopAncestorWithinFunc(stack[:len(stack)-1]) {
				warn("performance.go.defer-in-loop", fset.Position(node.Defer),
					"defer inside a loop runs only at function exit, so deferred resources accumulate each iteration; release explicitly or extract the loop body into a function")
			}
		case *ast.CallExpr:
			alias, name, ok := packageCall(node)
			if !ok {
				return true
			}
			inLoop := hasLoopAncestor(stack[:len(stack)-1])
			pos := fset.Position(node.Pos())
			switch {
			case inLoop && detectRegex && aliasHas(regexAliases, alias) && nameIn(regexCompileNames, name) && literalPatternArg(node):
				warn("performance.regex-compile-in-loop", pos,
					"regular expression compiled inside a loop; compile it once before the loop or as a package-level variable")
			// Test files are exempt from the sleep rule: polling with a short
			// sleep between readiness probes is the idiomatic test pattern.
			case inLoop && detectSleep && !strings.HasSuffix(file, "_test.go") && aliasHas(timeAliases, alias) && name == "Sleep":
				warn("performance.go.sleep-in-loop", pos,
					"time.Sleep inside a loop usually marks polling; prefer a time.Ticker, a channel signal, or a backoff helper")
			case inLoop && detectTimer && aliasHas(timeAliases, alias) && name == "After":
				warn("performance.go.timer-leak-in-loop", pos,
					"time.After inside a loop allocates a timer every iteration that is not collected until it fires; reuse a time.NewTimer or time.NewTicker")
			case detectReads && aliasHas(readAliases, alias) && name == "ReadAll" && !readerIsLimited(node):
				fn := enclosingFunc(stack[:len(stack)-1])
				if inLoop || (fn != nil && isLikelyHTTPHandler(fn, httpAliases)) {
					warn("performance.unbounded-read", pos,
						"ReadAll loads the entire input into memory; bound it with io.LimitReader or process the stream incrementally")
				}
			}
		}
		return true
	})
	return findings
}

// hasLoopAncestorWithinFunc reports a loop ancestor reached without crossing
// a function-literal boundary.
func hasLoopAncestorWithinFunc(stack []ast.Node) bool {
	for i := len(stack) - 1; i >= 0; i-- {
		switch stack[i].(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			return true
		case *ast.FuncLit, *ast.FuncDecl:
			return false
		}
	}
	return false
}

// literalPatternArg reports whether the compile call's pattern argument is a
// string literal. A variable pattern usually changes per iteration (compiling
// config-supplied patterns in a loop over them), which is not the smell.
func literalPatternArg(call *ast.CallExpr) bool {
	if len(call.Args) == 0 {
		return false
	}
	lit, ok := call.Args[0].(*ast.BasicLit)
	return ok && lit.Kind == token.STRING
}

// readerIsLimited reports whether a ReadAll argument already applies a bound
// (io.LimitReader / io.LimitedReader / http.MaxBytesReader), so the
// recommended fix is not itself flagged.
func readerIsLimited(call *ast.CallExpr) bool {
	limited := false
	for _, arg := range call.Args {
		ast.Inspect(arg, func(node ast.Node) bool {
			switch value := node.(type) {
			case *ast.SelectorExpr:
				switch value.Sel.Name {
				case "LimitReader", "LimitedReader", "MaxBytesReader":
					limited = true
				}
			case *ast.Ident:
				switch value.Name {
				case "LimitReader", "LimitedReader", "MaxBytesReader":
					limited = true
				}
			}
			return !limited
		})
		if limited {
			break
		}
	}
	return limited
}

// packageCall unpacks a pkg.Func selector call into its package alias and
// function name; method calls on non-identifier receivers return ok=false.
func packageCall(call *ast.CallExpr) (alias string, name string, ok bool) {
	sel, isSel := call.Fun.(*ast.SelectorExpr)
	if !isSel {
		return "", "", false
	}
	ident, isIdent := sel.X.(*ast.Ident)
	if !isIdent {
		return "", "", false
	}
	return ident.Name, sel.Sel.Name, true
}

func aliasHas(aliases map[string]struct{}, alias string) bool {
	_, ok := aliases[alias]
	return ok
}

func nameIn(names map[string]struct{}, name string) bool {
	_, ok := names[name]
	return ok
}
