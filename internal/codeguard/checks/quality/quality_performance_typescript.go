package quality

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
		env:      env,
		file:     file,
		limited:  tsConcurrencyLimitHint.MatchString(source),
		rules:    env.Config.Checks.QualityRules,
		findings: make([]core.Finding, 0),
	}
	for idx, line := range strings.Split(code, "\n") {
		scan.consumeLine(idx+1, line)
	}
	return scan.findings
}

type tsPerformanceScan struct {
	env      support.Context
	file     string
	limited  bool
	rules    core.QualityRulesConfig
	depth    int
	loops    []int
	handlers []int
	findings []core.Finding
}

func (s *tsPerformanceScan) consumeLine(lineNo int, line string) {
	startsLoop := tsLoopStartPattern.MatchString(line)
	startsHandler := tsHandlerStartPattern.MatchString(line)
	s.checkLine(lineNo, line, len(s.loops) > 0 || startsLoop, len(s.handlers) > 0 || startsHandler)
	next := s.depth + strings.Count(line, "{") - strings.Count(line, "}")
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

func (s *tsPerformanceScan) checkLine(lineNo int, line string, inLoop bool, inHandler bool) {
	if inLoop && qualityToggleEnabled(s.rules.DetectNPlusOneQuery) && tsQueryCallPattern.MatchString(line) {
		s.addFinding("quality.n-plus-one-query", "quality.n-plus-one-query", lineNo,
			"query or fetch call inside a loop suggests an N+1 pattern; batch requests or hoist the call out of the loop")
	}
	if inLoop && qualityToggleEnabled(s.rules.DetectUnboundedConcurrency) && !s.limited &&
		!strings.Contains(line, "await ") && tsPromiseCreatePattern.MatchString(line) {
		s.addFinding("quality.typescript.unbounded-concurrency", "quality.javascript.unbounded-concurrency", lineNo,
			"promise created inside a loop without a concurrency limit; batch with Promise.all over chunks or use p-limit")
	}
	if inHandler && qualityToggleEnabled(s.rules.DetectSyncIOInHandlers) && tsSyncCallPattern.MatchString(line) {
		s.addFinding("quality.typescript.sync-io-in-handler", "quality.javascript.sync-io-in-handler", lineNo,
			"synchronous I/O call inside a request handler blocks the event loop; use the async API instead")
	}
}

func (s *tsPerformanceScan) addFinding(tsRuleID string, jsRuleID string, lineNo int, message string) {
	s.findings = append(s.findings, warnFinding(s.env, support.RuleIDForScript(s.file, tsRuleID, jsRuleID), s.file, lineNo, 1, message))
}
