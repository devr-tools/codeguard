package performance

import (
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	tsLoopStartPattern     = regexp.MustCompile(`(?:^|[^\w$])(?:for|while)\s*\(|\.(?:forEach|map|flatMap)\s*\(`)
	tsHandlerStartPattern  = regexp.MustCompile(`\b(?:app|router|server|api|fastify)\.(?:get|post|put|delete|patch|all|use)\s*\(|export\s+(?:default\s+)?(?:async\s+)?function\s+handler\s*\(|export\s+(?:async\s+)?function\s+(?:GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\s*\(`)
	tsQueryCallPattern     = regexp.MustCompile(`\bfetch\s*\(|\baxios\b|\.query\s*\(|\.execute\s*\(|\.findOne\s*\(|\.findMany\s*\(|\.findUnique\s*\(|\.findFirst\s*\(`)
	tsSyncCallPattern      = regexp.MustCompile(`\b\w+Sync\s*\(`)
	tsPromiseCreatePattern = regexp.MustCompile(`new\s+Promise\s*\(|\.push\s*\(\s*(?:fetch\s*\(|axios\b)`)
	tsConcurrencyLimitHint = regexp.MustCompile(`p-limit|p-queue|pLimit\s*\(`)
	tsAwaitPattern         = regexp.MustCompile(`(?:^|[^\w$])await\s`)
	tsForAwaitPattern      = regexp.MustCompile(`\bfor\s+await\s*\(`)
	// tsRegexCompilePattern requires a literal pattern (matched on the raw
	// line, since the stripper blanks quotes): a variable argument usually
	// varies per iteration, which is not the hoistable smell.
	tsRegexCompilePattern = regexp.MustCompile(`new\s+RegExp\s*\(\s*["'` + "`" + `]`)
	// tsStringConcat matches "name += <string-ish>": a quote, backtick, or
	// String(...) start keeps numeric accumulators out. It runs on the RAW
	// line (the stripper blanks quote delimiters), guarded by the stripped
	// line still containing += so comment text cannot match. Augmented
	// assignment to a variable initialized from a string literal (tracked by
	// tsStringInit) is caught separately, covering out += row + "\n".
	tsStringConcat        = regexp.MustCompile(`[\w$\]]\s*\+=\s*(?:["'` + "`" + `]|String\s*\()`)
	tsStringInit          = regexp.MustCompile(`\b(?:let|var)\s+([\w$]+)\s*(?::\s*string\s*)?=\s*["'` + "`" + `]`)
	tsAugmentedAssign     = regexp.MustCompile(`(?:^|[^\w$.])([\w$]+)\s*\+=`)
	tsSetIntervalPattern  = regexp.MustCompile(`\bsetInterval\s*\(`)
	tsClearInterval       = regexp.MustCompile(`\bclearInterval\s*\(`)
	tsAddListenerPattern  = regexp.MustCompile(`\baddEventListener\s*\(`)
	tsRemoveListenerCall  = regexp.MustCompile(`\bremoveEventListener\s*\(`)
	tsAbortSignalListener = regexp.MustCompile(`\bsignal\s*[:=]`)
)

func typeScriptPerformanceTargetFindings(env support.Context, target core.TargetConfig) []core.Finding {
	findings := make([]core.Finding, 0)
	env.VisitTargetFiles(target, isTypeScriptLikeFile, func(rel string, data []byte) {
		findings = append(findings, typeScriptPerformanceFindings(env, rel, data)...)
	})
	return findings
}

func typeScriptPerformanceFindings(env support.Context, file string, data []byte) []core.Finding {
	source := strings.ReplaceAll(string(data), "\r\n", "\n")
	code := support.StripTypeScriptCommentsAndStrings(source)
	scan := &tsPerformanceScan{
		env:              env,
		file:             file,
		limited:          tsConcurrencyLimitHint.MatchString(source),
		listenersCleaned: tsRemoveListenerCall.MatchString(code) || tsAbortSignalListener.MatchString(code),
		rules:            env.Config.Checks.PerformanceRules,
		frameworks:       newTSFrameworkScan(env.Config.Checks.PerformanceRules, source),
		findings:         make([]core.Finding, 0),
	}
	rawLines := strings.Split(source, "\n")
	for idx, line := range strings.Split(code, "\n") {
		scan.consumeLine(idx+1, line, rawLines[idx])
	}
	// setInterval leaks are a file-level judgment: any clearInterval in the
	// file counts as cleanup, so the findings are emitted after the pass.
	if toggleEnabled(scan.rules.DetectTimerLeaks) && !scan.intervalsCleaned {
		for _, lineNo := range scan.intervalLines {
			scan.addFinding("performance.typescript.timer-listener-leak", "performance.javascript.timer-listener-leak", lineNo,
				"setInterval without any clearInterval in the file keeps the callback and its captures alive forever; store the handle and clear it")
		}
	}
	return scan.findings
}

type tsPerformanceScan struct {
	env              support.Context
	file             string
	limited          bool
	listenersCleaned bool
	intervalsCleaned bool
	intervalLines    []int
	stringVars       map[string]struct{}
	rules            core.PerformanceRulesConfig
	frameworks       *tsFrameworkScan
	depth            int
	loops            []int
	handlers         []int
	findings         []core.Finding
}

func (s *tsPerformanceScan) consumeLine(lineNo int, line string, rawLine string) {
	if m := tsStringInit.FindStringSubmatch(rawLine); m != nil {
		if s.stringVars == nil {
			s.stringVars = map[string]struct{}{}
		}
		s.stringVars[m[1]] = struct{}{}
	}
	startsLoop := tsLoopStartPattern.MatchString(line)
	startsHandler := tsHandlerStartPattern.MatchString(line)
	s.checkLine(lineNo, line, rawLine, len(s.loops) > 0 || startsLoop, len(s.handlers) > 0 || startsHandler)
	next := s.depth + strings.Count(line, "{") - strings.Count(line, "}")
	s.frameworks.observe(s, lineNo, line, next)
	if startsLoop && next > s.depth {
		s.loops = append(s.loops, s.depth)
	}
	if startsHandler && next > s.depth {
		s.handlers = append(s.handlers, s.depth)
	}
	for len(s.loops) > 0 && next <= s.loops[len(s.loops)-1] {
		s.loops = s.loops[:len(s.loops)-1]
	}
	for len(s.handlers) > 0 && next <= s.handlers[len(s.handlers)-1] {
		s.handlers = s.handlers[:len(s.handlers)-1]
	}
	s.depth = next
}

func (s *tsPerformanceScan) checkLine(lineNo int, line string, rawLine string, inLoop bool, inHandler bool) {
	if tsSetIntervalPattern.MatchString(line) {
		s.intervalLines = append(s.intervalLines, lineNo)
	}
	if tsClearInterval.MatchString(line) {
		s.intervalsCleaned = true
	}
	if inLoop && toggleEnabled(s.rules.DetectNPlusOneQuery) && tsQueryCallPattern.MatchString(line) {
		s.addFinding("performance.n-plus-one-query", "performance.n-plus-one-query", lineNo,
			"query or fetch call inside a loop suggests an N+1 pattern; batch requests or hoist the call out of the loop")
	}
	if inLoop && toggleEnabled(s.rules.DetectUnboundedConcurrency) && !s.limited &&
		!strings.Contains(line, "await ") && tsPromiseCreatePattern.MatchString(line) {
		s.addFinding("performance.typescript.unbounded-concurrency", "performance.javascript.unbounded-concurrency", lineNo,
			"promise created inside a loop without a concurrency limit; batch with Promise.all over chunks or use p-limit")
	}
	if inHandler && tsSyncCallPattern.MatchString(line) {
		// The framework-aware express middleware rule takes precedence over
		// the generic sync-io rule so a single line never reports twice.
		if !s.frameworks.reportExpressSyncMiddleware(s, lineNo, line) && toggleEnabled(s.rules.DetectSyncIOInHandlers) {
			s.addFinding("performance.typescript.sync-io-in-handler", "performance.javascript.sync-io-in-handler", lineNo,
				"synchronous I/O call inside a request handler blocks the event loop; use the async API instead")
		}
	}
	if !inLoop {
		return
	}
	if toggleEnabled(s.rules.DetectAwaitInLoop) && !s.limited &&
		tsAwaitPattern.MatchString(line) && !tsForAwaitPattern.MatchString(line) {
		s.addFinding("performance.typescript.await-in-loop", "performance.javascript.await-in-loop", lineNo,
			"await inside a loop serializes independent work; collect the promises and await Promise.all over chunks")
	}
	if toggleEnabled(s.rules.DetectRegexCompileInLoop) && strings.Contains(line, "RegExp") &&
		tsRegexCompilePattern.MatchString(rawLine) {
		s.addFinding("performance.regex-compile-in-loop", "performance.regex-compile-in-loop", lineNo,
			"new RegExp inside a loop recompiles the pattern every iteration; hoist it out of the loop or use a literal")
	}
	if toggleEnabled(s.rules.DetectAllocInLoop) && strings.Contains(line, "+=") && s.isStringConcat(line, rawLine) {
		s.addFinding("performance.string-concat-in-loop", "performance.string-concat-in-loop", lineNo,
			"string built by += inside a loop copies the whole value each iteration; collect parts in an array and join them")
	}
	if toggleEnabled(s.rules.DetectTimerLeaks) && !s.listenersCleaned && tsAddListenerPattern.MatchString(line) {
		s.addFinding("performance.typescript.timer-listener-leak", "performance.javascript.timer-listener-leak", lineNo,
			"addEventListener inside a loop with no removeEventListener or AbortSignal in the file accumulates listeners; deduplicate or clean them up")
	}
}

// isStringConcat reports += growth of a string: either a string-ish
// right-hand side on the raw line, or an augmented assignment (on the
// stripped line, so comments cannot match) to a variable initialized from a
// string literal.
func (s *tsPerformanceScan) isStringConcat(line string, rawLine string) bool {
	if tsStringConcat.MatchString(rawLine) {
		return true
	}
	m := tsAugmentedAssign.FindStringSubmatch(line)
	if m == nil {
		return false
	}
	_, isString := s.stringVars[m[1]]
	return isString
}

func (s *tsPerformanceScan) addFinding(tsRuleID string, jsRuleID string, lineNo int, message string) {
	s.findings = append(s.findings, warnFinding(s.env, support.RuleIDForScript(s.file, tsRuleID, jsRuleID), s.file, lineNo, 1, message))
}
