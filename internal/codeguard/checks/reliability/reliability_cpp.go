package reliability

import (
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	cppLoopStart        = regexp.MustCompile(`(?:^|[^\w])(?:for|while)\b`)
	cppUnboundedLoop    = regexp.MustCompile(`\bwhile\s*\(\s*true\s*\)|\bfor\s*\(\s*;\s*;\s*\)`)
	cppRetryHint        = regexp.MustCompile(`(?i)retry|attempt|transient`)
	cppBackoffHint      = regexp.MustCompile(`(?i)sleep_for|sleep_until|backoff|jitter`)
	cppThreadLaunch     = regexp.MustCompile(`\bstd::(?:thread|jthread|async)\s*\(`)
	cppConcurrencyLimit = regexp.MustCompile(`semaphore|latch|barrier|thread_pool|executor|queue`)
	cppCancellationHint = regexp.MustCompile(`(?i)\b(?:stop_token|stop_source|cancellation|cancel|deadline|timeout)\b`)
	cppShutdownHint     = regexp.MustCompile(`(?i)\b(?:SIGTERM|SIGINT|signal\(|sigaction|Shutdown|Stop|Drain|Graceful|join\(\)|request_stop)\b`)
	cppServerStart      = regexp.MustCompile(`(?i)\.(?:Listen|Serve|Run|Start)\s*\(`)
	cppOutboundCall     = regexp.MustCompile(`(?i)\.(?:Get|Post|Put|Patch|Delete|Fetch|Request|Send|Execute)\s*\(`)
	cppRawNew           = regexp.MustCompile(`\bnew\s+[A-Za-z_:]\w*`)
	cppDeleteCall       = regexp.MustCompile(`\bdelete\s+`)
	cppThrowRuntime     = regexp.MustCompile(`\bthrow\s+std::(?:runtime_error|exception)\s*\(`)
	cppCatchStart       = regexp.MustCompile(`\bcatch\s*\([^)]*(?:exception|Error|Status)[^)]*(?:\berr\b|\berror\b|\be\b)[^)]*\)`)
	cppLostContextThrow = regexp.MustCompile(`\bthrow\s+std::(?:runtime_error|logic_error|exception)\s*\(`)
	cppSwallowedCatch   = regexp.MustCompile(`^\s*(?:return\s*;|continue\s*;|break\s*;)\s*$`)
	cppNonIdempotent    = regexp.MustCompile(`(?i)\b(?:post|put|patch|delete|create|update|save|insert|publish|send|charge|write)\w*\s*\(`)
	cppIdempotency      = regexp.MustCompile(`(?i)idempot|dedupe|dedup|processed|message_id|event_id`)
)

func cppFindingsForFile(env support.Context, file string, data []byte) []core.Finding {
	source := strings.ReplaceAll(string(data), "\r\n", "\n")
	masked := support.MaskCLikeSource(source, support.CLikeCPP)
	scan := &cppReliabilityScan{
		env:         env,
		file:        file,
		rules:       env.Config.Checks.ReliabilityRules,
		limited:     cppConcurrencyLimit.MatchString(masked),
		cancellable: cppCancellationHint.MatchString(masked),
		hasShutdown: cppShutdownHint.MatchString(masked),
		catchDepth:  -1,
	}
	for idx, line := range strings.Split(masked, "\n") {
		scan.consumeLine(idx+1, line)
	}
	scan.finish()
	scan.findings = append(scan.findings, partialFailureHiddenFindings(env, file, data)...)
	return scan.findings
}

type cppReliabilityScan struct {
	env            support.Context
	file           string
	rules          core.ReliabilityRulesConfig
	limited        bool
	cancellable    bool
	hasShutdown    bool
	depth          int
	loops          []int
	unboundedLoops []int
	newLine        int
	threadLines    []int
	catchDepth     int
	findings       []core.Finding
}

func (s *cppReliabilityScan) consumeLine(lineNo int, line string) {
	startsLoop := cppLoopStart.MatchString(line)
	inLoop := len(s.loops) > 0 || startsLoop
	inUnboundedLoop := len(s.unboundedLoops) > 0 || cppUnboundedLoop.MatchString(line)
	inCatch := s.catchDepth >= 0 || cppCatchStart.MatchString(line)
	s.checkLine(lineNo, line, inLoop, inUnboundedLoop)
	if enabled(s.rules.DetectLostErrorContext) && inCatch && cppLostContextThrow.MatchString(line) && !strings.Contains(line, "what()") {
		s.add("reliability.lost-error-context", "warn", lineNo, "catch block replaces the original exception without preserving diagnostic context", "medium", "error", "throw-runtime-error")
	}
	if enabled(s.rules.DetectSwallowedError) && inCatch && cppSwallowedCatch.MatchString(line) {
		s.add("reliability.swallowed-error", "fail", lineNo, "catch block exits without surfacing or preserving the caught exception", "high", "error", "catch-swallowed")
	}
	next := s.depth + strings.Count(line, "{") - strings.Count(line, "}")
	if cppCatchStart.MatchString(line) {
		s.catchDepth = next - 1
		if s.catchDepth < 0 {
			s.catchDepth = 0
		}
	}
	if startsLoop && next > s.depth {
		s.loops = append(s.loops, s.depth)
		if cppUnboundedLoop.MatchString(line) {
			s.unboundedLoops = append(s.unboundedLoops, s.depth)
		}
	}
	for len(s.loops) > 0 && next <= s.loops[len(s.loops)-1] {
		s.loops = s.loops[:len(s.loops)-1]
	}
	for len(s.unboundedLoops) > 0 && next <= s.unboundedLoops[len(s.unboundedLoops)-1] {
		s.unboundedLoops = s.unboundedLoops[:len(s.unboundedLoops)-1]
	}
	if s.catchDepth >= 0 && next <= s.catchDepth {
		s.catchDepth = -1
	}
	s.depth = next
}

func (s *cppReliabilityScan) checkLine(lineNo int, line string, inLoop bool, inUnboundedLoop bool) {
	if enabled(s.rules.DetectMissingTimeout) && cppOutboundCall.MatchString(line) && !strings.Contains(strings.ToLower(line), "timeout") && !strings.Contains(strings.ToLower(line), "deadline") {
		s.add("reliability.missing-timeout", "fail", lineNo, "outbound C++ dependency call has no visible timeout or deadline", "medium", "call", "dependency-without-timeout")
	}
	if enabled(s.rules.DetectRetryWithoutBackoff) && inLoop && cppRetryHint.MatchString(line) && !cppBackoffHint.MatchString(line) {
		s.add("reliability.retry-without-backoff", "warn", lineNo, "retry-like C++ loop has no visible backoff or jitter", "medium", "retry", "no-backoff")
	}
	if enabled(s.rules.DetectUnboundedRetry) && inUnboundedLoop && cppRetryHint.MatchString(line) {
		s.add("reliability.unbounded-retry", "fail", lineNo, "retry-like C++ loop can run forever without an attempt limit", "medium", "retry", "while-true")
	}
	if enabled(s.rules.DetectNonIdempotentRetry) && inLoop && cppNonIdempotent.MatchString(line) && !cppIdempotency.MatchString(line) {
		s.add("reliability.non-idempotent-retry", "fail", lineNo, "retry-like C++ loop wraps a non-idempotent side effect without idempotency evidence", "medium", "retry", "side-effect")
	}
	if enabled(s.rules.DetectUnboundedWork) && inLoop && !s.limited && cppThreadLaunch.MatchString(line) {
		s.add("reliability.unbounded-work", "warn", lineNo, "C++ thread/task is launched inside a loop without a visible concurrency bound", "high", "work", "thread-in-loop")
	}
	if cppThreadLaunch.MatchString(line) {
		s.threadLines = append(s.threadLines, lineNo)
		if enabled(s.rules.DetectMissingCancellation) && !s.cancellable {
			s.add("reliability.missing-cancellation", "warn", lineNo, "C++ async work starts without visible stop_token, deadline, or cancellation propagation", "medium", "context", "cpp-async-work")
		}
	}
	if enabled(s.rules.DetectMissingGracefulShutdown) && cppServerStart.MatchString(line) && !s.hasShutdown {
		s.add("reliability.missing-graceful-shutdown", "warn", lineNo, "C++ service starts without visible signal handling, stop, drain, or graceful shutdown path", "medium", "server", "cpp-server-start")
	}
	if enabled(s.rules.DetectRecoverablePanic) && cppThrowRuntime.MatchString(line) {
		s.add("reliability.recoverable-panic", "fail", lineNo, "production C++ code throws a generic runtime exception for a recoverable failure path", "medium", "exception", "runtime-error")
	}
	if enabled(s.rules.DetectResourceLeak) {
		s.trackRawNew(lineNo, line)
	}
}

func (s *cppReliabilityScan) finish() {
	limit := s.rules.MaxInlineGoroutinesPerFunction
	if limit <= 0 {
		limit = 4
	}
	if enabled(s.rules.DetectMissingConcurrencyLimit) && !s.limited && len(s.threadLines) > limit {
		s.add("reliability.missing-concurrency-limit", "warn", s.threadLines[0], "file starts multiple C++ threads/tasks without an obvious concurrency limit", "medium", "tasks", "cpp-thread-launches")
	}
}

func (s *cppReliabilityScan) trackRawNew(lineNo int, line string) {
	if strings.Contains(line, "unique_ptr") || strings.Contains(line, "shared_ptr") || strings.Contains(line, "make_unique") || strings.Contains(line, "make_shared") {
		return
	}
	if cppRawNew.MatchString(line) {
		s.newLine = lineNo
		return
	}
	if s.newLine > 0 && cppDeleteCall.MatchString(line) {
		s.newLine = 0
	}
	if s.newLine > 0 && lineNo > s.newLine+8 {
		s.add("reliability.resource-leak", "fail", s.newLine, "raw C++ allocation has no nearby delete or smart-pointer ownership evidence", "medium", "resource", "raw-new")
		s.newLine = 0
	}
}

func (s *cppReliabilityScan) add(ruleID string, level string, lineNo int, message string, confidence string, metaKey string, metaValue string) {
	s.findings = append(s.findings, newFinding(s.env, ruleID, level, s.file, lineNo, 1, message, confidence, metaKey, metaValue))
}
