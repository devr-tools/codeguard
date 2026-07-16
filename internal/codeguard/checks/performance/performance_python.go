package performance

import (
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	pythonLoopStartPattern = regexp.MustCompile(`^\s*(?:for\s+.+\s+in\s+.+:|while\s+.+:)`)
	pythonAsyncDefPattern  = regexp.MustCompile(`^\s*async\s+def\s+`)
	pythonQueryCallPattern = regexp.MustCompile(`\b(?:requests|httpx)\.(?:get|post|put|delete|patch|head)\s*\(|\bcursor\.execute\s*\(|\.execute\s*\(|\bsession\.query\s*\(`)
	pythonSyncInAsyncCall  = regexp.MustCompile(`\brequests\.\w+\s*\(|\burllib\.request\.urlopen\s*\(|\btime\.sleep\s*\(`)
	// pythonRegexCompileCall requires a literal pattern: a variable argument
	// usually varies per iteration, which is not the hoistable smell.
	pythonRegexCompileCall = regexp.MustCompile(`\bre\.compile\s*\(\s*[frbu]{0,2}["']`)
	// pythonStringConcat matches "name += <string-ish>": a quote, f-string, or
	// str(...) on the right-hand side keeps integer accumulators out. Augmented
	// assignment to a variable initialized from a string literal (tracked by
	// pythonStringAssign) is caught separately, covering out += line.
	pythonStringConcat    = regexp.MustCompile(`^\s*\w+\s*\+=\s*(?:[frbu]{0,2}["']|str\s*\()`)
	pythonStringAssign    = regexp.MustCompile(`^\s*(\w+)\s*=\s*[frbu]{0,2}["']`)
	pythonAugmentedAssign = regexp.MustCompile(`^\s*(\w+)\s*\+=`)
	pythonTaskCreateCall  = regexp.MustCompile(`\basyncio\.(?:create_task|ensure_future)\s*\(`)
	pythonUnboundedRead   = regexp.MustCompile(`\.(?:read|readlines)\s*\(\s*\)`)
	pythonConcurrencyHint = regexp.MustCompile(`Semaphore\s*\(|TaskGroup\s*\(|aiolimiter|anyio\.CapacityLimiter`)
)

func pythonPerformanceFindings(env support.Context, file string, data []byte) []core.Finding {
	source := strings.ReplaceAll(string(data), "\r\n", "\n")
	scan := &pythonPerformanceScan{
		env:        env,
		file:       file,
		rules:      env.Config.Checks.PerformanceRules,
		limited:    pythonConcurrencyHint.MatchString(source),
		frameworks: newPythonFrameworkScan(env.Config.Checks.PerformanceRules, source),
	}
	for idx, line := range strings.Split(source, "\n") {
		scan.consumeLine(idx+1, line)
	}
	return scan.findings
}

type pythonPerformanceScan struct {
	env        support.Context
	file       string
	rules      core.PerformanceRulesConfig
	limited    bool
	stringVars map[string]struct{}
	loops      []int
	asyncDefs  []int
	frameworks *pythonFrameworkScan
	findings   []core.Finding
}

func (s *pythonPerformanceScan) consumeLine(lineNo int, line string) {
	if strings.TrimSpace(line) == "" {
		return
	}
	if m := pythonStringAssign.FindStringSubmatch(line); m != nil {
		if s.stringVars == nil {
			s.stringVars = map[string]struct{}{}
		}
		s.stringVars[m[1]] = struct{}{}
	}
	indent := indentationWidth(line)
	s.loops = popIndentRegions(s.loops, indent)
	s.asyncDefs = popIndentRegions(s.asyncDefs, indent)
	startsLoop := pythonLoopStartPattern.MatchString(line)
	s.checkLine(lineNo, line, len(s.loops) > 0 || startsLoop, len(s.asyncDefs) > 0)
	s.frameworks.observe(s, lineNo, line, indent, len(s.loops) > 0 || startsLoop)
	if startsLoop {
		s.loops = append(s.loops, indent)
	}
	if pythonAsyncDefPattern.MatchString(line) {
		s.asyncDefs = append(s.asyncDefs, indent)
	}
}

func (s *pythonPerformanceScan) checkLine(lineNo int, line string, inLoop bool, inAsync bool) {
	if inLoop && toggleEnabled(s.rules.DetectNPlusOneQuery) && pythonQueryCallPattern.MatchString(line) {
		s.addFinding("performance.n-plus-one-query", lineNo,
			"query or request call inside a loop suggests an N+1 pattern; batch the work or hoist the call out of the loop")
	}
	if inAsync && toggleEnabled(s.rules.DetectSyncIOInHandlers) && pythonSyncInAsyncCall.MatchString(line) {
		s.addFinding("performance.python.sync-io-in-async", lineNo,
			"blocking call inside an async function stalls the event loop; use an async client or asyncio.sleep")
	}
	if !inLoop {
		return
	}
	if toggleEnabled(s.rules.DetectRegexCompileInLoop) && pythonRegexCompileCall.MatchString(line) {
		s.addFinding("performance.regex-compile-in-loop", lineNo,
			"re.compile inside a loop recompiles the pattern every iteration; compile it once at module level or before the loop")
	}
	if toggleEnabled(s.rules.DetectAllocInLoop) && s.isStringConcat(line) {
		s.addFinding("performance.string-concat-in-loop", lineNo,
			"string built by += inside a loop copies the whole value each iteration; collect parts in a list and \"\".join them")
	}
	if toggleEnabled(s.rules.DetectUnboundedConcurrency) && !s.limited && pythonTaskCreateCall.MatchString(line) {
		s.addFinding("performance.python.unbounded-concurrency", lineNo,
			"asyncio task created inside a loop without a concurrency limit; bound it with asyncio.Semaphore or a TaskGroup")
	}
	if toggleEnabled(s.rules.DetectUnboundedReads) && pythonUnboundedRead.MatchString(line) {
		s.addFinding("performance.unbounded-read", lineNo,
			"unbounded read() inside a loop loads whole inputs into memory; read in chunks or iterate the stream")
	}
}

// isStringConcat reports += growth of a string: either a string-ish
// right-hand side, or an augmented assignment to a variable that was earlier
// initialized from a string literal.
func (s *pythonPerformanceScan) isStringConcat(line string) bool {
	if pythonStringConcat.MatchString(line) {
		return true
	}
	m := pythonAugmentedAssign.FindStringSubmatch(line)
	if m == nil {
		return false
	}
	_, isString := s.stringVars[m[1]]
	return isString
}

func (s *pythonPerformanceScan) addFinding(ruleID string, lineNo int, message string) {
	s.findings = append(s.findings, warnFinding(s.env, ruleID, s.file, lineNo, 1, message))
}

func popIndentRegions(regions []int, indent int) []int {
	for len(regions) > 0 && indent <= regions[len(regions)-1] {
		regions = regions[:len(regions)-1]
	}
	return regions
}
func indentationWidth(line string) int {
	width := 0
	for _, ch := range line {
		if ch == ' ' {
			width++
			continue
		}
		if ch == '\t' {
			width += 4
			continue
		}
		break
	}
	return width
}
