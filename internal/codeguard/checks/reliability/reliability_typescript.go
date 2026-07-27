package reliability

import (
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	tsFetchCall         = regexp.MustCompile(`\bfetch\s*\(`)
	tsAxiosCall         = regexp.MustCompile(`\baxios\.(?:get|post|put|patch|delete)\s*\(`)
	tsLoopStart         = regexp.MustCompile(`(?:^|[^\w$])(?:for|while)\s*\(|\.(?:forEach|map|flatMap)\s*\(`)
	tsUnboundedLoop     = regexp.MustCompile(`\bwhile\s*\(\s*true\s*\)|\bfor\s*\(\s*;\s*;\s*\)`)
	tsRetryHint         = regexp.MustCompile(`(?i)retry|attempt|transient`)
	tsBackoffHint       = regexp.MustCompile(`(?i)backoff|jitter|setTimeout|sleep|delay`)
	tsPromiseInLoop     = regexp.MustCompile(`\b(?:new\s+Promise|fetch|axios\.|Promise\.all|[A-Za-z_$][\w$]*Async|fetch[A-Za-z_$][\w$]*)\s*\(`)
	tsLimitHint         = regexp.MustCompile(`p-limit|pLimit|Bottleneck|PQueue|Semaphore|pool|queue|limit\s*\(`)
	tsCancellationHint  = regexp.MustCompile(`AbortSignal|AbortController|signal\s*:|signal\s*,|signal\s*\}|timeout|clearTimeout|controller\.abort`)
	tsServerListen      = regexp.MustCompile(`(?:\bapp|\bserver|createServer\s*\([^)]*\))\.listen\s*\(`)
	tsShutdownHint      = regexp.MustCompile(`(?i)SIGTERM|SIGINT|beforeExit|process\.on\s*\(|server\.close|shutdown|graceful|drain`)
	tsDetachedPromise   = regexp.MustCompile(`^\s*(?:void\s+)?(?:fetch|fetch[A-Z][A-Za-z0-9_$]*|axios\.|[A-Za-z_$][\w$]*Async)\s*\(`)
	tsSwallowedCatch    = regexp.MustCompile(`catch\s*\([^)]*\)\s*\{\s*(?:return\s+undefined\s*;?|return\s*;?|console\.(?:log|warn|error)\([^)]*\)\s*;?)?\s*\}`)
	tsLostContextCatch  = regexp.MustCompile(`catch\s*\((?:err|error|e)\)\s*\{\s*throw\s+new\s+(?:Error|TypeError|RangeError)\s*\(`)
	tsResourceOpen      = regexp.MustCompile(`\b(?:fs\.openSync|fs\.createReadStream|fs\.createWriteStream|createReadStream|createWriteStream)\s*\(`)
	tsResourceClose     = regexp.MustCompile(`\.(?:close|destroy)\s*\(`)
	tsGenericThrow      = regexp.MustCompile(`throw\s+new\s+(?:Error|TypeError|RuntimeError)\s*\(`)
	tsNonIdempotentCall = regexp.MustCompile(`(?i)\b(?:post|put|patch|delete|create|update|save|insert|publish|send|charge|write)\w*\s*\(`)
	tsIdempotencyHint   = regexp.MustCompile(`(?i)idempot|dedupe|dedup|processed|messageId|eventId`)
)

func typeScriptTargetFindings(env support.Context, target core.TargetConfig) []core.Finding {
	findings := make([]core.Finding, 0)
	env.VisitTargetFiles(target, support.IsTypeScriptLikeFile, func(rel string, data []byte) {
		findings = append(findings, typeScriptFindingsForFile(env, rel, data)...)
	})
	return findings
}

func typeScriptFindingsForFile(env support.Context, file string, data []byte) []core.Finding {
	source := strings.ReplaceAll(string(data), "\r\n", "\n")
	code := support.StripTypeScriptCommentsAndStrings(source)
	scan := &tsReliabilityScan{
		env:         env,
		file:        file,
		rules:       env.Config.Checks.ReliabilityRules,
		limited:     tsLimitHint.MatchString(source),
		cancellable: tsCancellationHint.MatchString(source),
		hasShutdown: tsShutdownHint.MatchString(source),
	}
	for idx, line := range strings.Split(code, "\n") {
		scan.consumeLine(idx+1, line)
	}
	scan.finish()
	if enabled(scan.rules.DetectSwallowedError) {
		for idx, rawLine := range strings.Split(source, "\n") {
			if tsSwallowedCatch.MatchString(rawLine) {
				scan.add("reliability.swallowed-error", "fail", idx+1, "catch block swallows an error without returning or propagating it", "high", "error", "catch-swallowed")
			}
			if enabled(scan.rules.DetectLostErrorContext) && tsLostContextCatch.MatchString(rawLine) {
				scan.add("reliability.lost-error-context", "warn", idx+1, "catch block replaces the original error without preserving it as cause", "medium", "error", "throw-new-error")
			}
		}
	}
	scan.findings = append(scan.findings, partialFailureHiddenFindings(env, file, data)...)
	return scan.findings
}

type tsReliabilityScan struct {
	env            support.Context
	file           string
	rules          core.ReliabilityRulesConfig
	limited        bool
	cancellable    bool
	hasShutdown    bool
	depth          int
	loops          []int
	unboundedLoops []int
	detachedLines  []int
	resourceLine   int
	findings       []core.Finding
}

func (s *tsReliabilityScan) consumeLine(lineNo int, line string) {
	startsLoop := tsLoopStart.MatchString(line)
	inLoop := len(s.loops) > 0 || startsLoop
	inUnboundedLoop := len(s.unboundedLoops) > 0 || tsUnboundedLoop.MatchString(line)
	s.checkLine(lineNo, line, inLoop, inUnboundedLoop)
	next := s.depth + strings.Count(line, "{") - strings.Count(line, "}")
	if startsLoop && next > s.depth {
		s.loops = append(s.loops, s.depth)
		if tsUnboundedLoop.MatchString(line) {
			s.unboundedLoops = append(s.unboundedLoops, s.depth)
		}
	}
	for len(s.loops) > 0 && next <= s.loops[len(s.loops)-1] {
		s.loops = s.loops[:len(s.loops)-1]
	}
	for len(s.unboundedLoops) > 0 && next <= s.unboundedLoops[len(s.unboundedLoops)-1] {
		s.unboundedLoops = s.unboundedLoops[:len(s.unboundedLoops)-1]
	}
	s.depth = next
}

func (s *tsReliabilityScan) checkLine(lineNo int, line string, inLoop bool, inUnboundedLoop bool) {
	if enabled(s.rules.DetectMissingTimeout) && (tsFetchCall.MatchString(line) || tsAxiosCall.MatchString(line)) && !strings.Contains(line, "timeout") && !strings.Contains(line, "signal") {
		s.add("reliability.missing-timeout", "fail", lineNo, "outbound TypeScript/JavaScript HTTP call lacks timeout or abort signal evidence", "medium", "call", "http-without-timeout")
	}
	if enabled(s.rules.DetectUnboundedWork) && inLoop && !s.limited && tsPromiseInLoop.MatchString(line) {
		s.add("reliability.unbounded-work", "warn", lineNo, "promise or HTTP work starts inside a loop without a visible concurrency limit", "medium", "work", "promise-in-loop")
	}
	if tsDetachedPromise.MatchString(line) {
		s.detachedLines = append(s.detachedLines, lineNo)
		if enabled(s.rules.DetectMissingCancellation) && !s.cancellable {
			s.add("reliability.missing-cancellation", "warn", lineNo, "detached async work starts without visible AbortSignal, timeout, or cancellation propagation", "medium", "context", "detached-promise")
		}
	}
	if enabled(s.rules.DetectMissingGracefulShutdown) && tsServerListen.MatchString(line) && !s.hasShutdown {
		s.add("reliability.missing-graceful-shutdown", "warn", lineNo, "JavaScript server starts without visible signal handling or server.close shutdown path", "medium", "server", "listen")
	}
	if enabled(s.rules.DetectResourceLeak) {
		s.trackResourceLeak(lineNo, line)
	}
	if enabled(s.rules.DetectRetryWithoutBackoff) && inLoop && tsRetryHint.MatchString(line) && !tsBackoffHint.MatchString(line) {
		s.add("reliability.retry-without-backoff", "warn", lineNo, "retry-like JavaScript loop has no visible backoff or jitter", "medium", "retry", "no-backoff")
	}
	if enabled(s.rules.DetectUnboundedRetry) && inUnboundedLoop && tsRetryHint.MatchString(line) {
		s.add("reliability.unbounded-retry", "fail", lineNo, "retry-like JavaScript loop can run forever without an attempt limit", "medium", "retry", "while-true")
	}
	if enabled(s.rules.DetectNonIdempotentRetry) && inLoop && tsNonIdempotentCall.MatchString(line) && !tsIdempotencyHint.MatchString(line) {
		s.add("reliability.non-idempotent-retry", "fail", lineNo, "retry-like JavaScript loop wraps a non-idempotent side effect without idempotency evidence", "medium", "retry", "side-effect")
	}
	if enabled(s.rules.DetectRecoverablePanic) && tsGenericThrow.MatchString(line) {
		s.add("reliability.recoverable-panic", "fail", lineNo, "production code throws a generic exception for a recoverable failure path", "medium", "exception", "generic-throw")
	}
}

func (s *tsReliabilityScan) trackResourceLeak(lineNo int, line string) {
	if tsResourceOpen.MatchString(line) {
		s.resourceLine = lineNo
	}
	if s.resourceLine > 0 && tsResourceClose.MatchString(line) {
		s.resourceLine = 0
	}
	if s.resourceLine > 0 && lineNo > s.resourceLine+8 {
		s.add("reliability.resource-leak", "fail", s.resourceLine, "Node.js file or stream resource is not closed near the acquisition path", "medium", "resource", "node-stream")
		s.resourceLine = 0
	}
}

func (s *tsReliabilityScan) finish() {
	limit := s.rules.MaxInlineGoroutinesPerFunction
	if limit <= 0 {
		limit = 4
	}
	if enabled(s.rules.DetectMissingConcurrencyLimit) && !s.limited && len(s.detachedLines) > limit {
		s.add("reliability.missing-concurrency-limit", "warn", s.detachedLines[0], "file starts multiple detached async operations without an obvious concurrency limit", "medium", "tasks", "detached-promises")
	}
}

func (s *tsReliabilityScan) add(ruleID string, level string, lineNo int, message string, confidence string, metaKey string, metaValue string) {
	s.findings = append(s.findings, newFinding(s.env, ruleID, level, s.file, lineNo, 1, message, confidence, metaKey, metaValue))
}
