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
	scope := lineScope{
		inLoop:          len(s.loops) > 0 || startsLoop,
		inUnboundedLoop: len(s.unboundedLoops) > 0 || cppUnboundedLoop.MatchString(line),
		inHandler:       s.catchDepth >= 0 || cppCatchStart.MatchString(line),
	}
	s.checkLine(lineNo, line, scope)
	if scope.inHandler {
		s.checkCatchLine(lineNo, line)
	}
	next := s.depth + strings.Count(line, "{") - strings.Count(line, "}")
	if cppCatchStart.MatchString(line) {
		s.catchDepth = max(next-1, 0)
	}
	if startsLoop && next > s.depth {
		s.loops = append(s.loops, s.depth)
		if cppUnboundedLoop.MatchString(line) {
			s.unboundedLoops = append(s.unboundedLoops, s.depth)
		}
	}
	s.loops = popBlockRegions(s.loops, next)
	s.unboundedLoops = popBlockRegions(s.unboundedLoops, next)
	if s.catchDepth >= 0 && next <= s.catchDepth {
		s.catchDepth = -1
	}
	s.depth = next
}

// checkCatchLine reports catch blocks that rethrow without the original
// exception's context or exit without surfacing it.
func (s *cppReliabilityScan) checkCatchLine(lineNo int, line string) {
	if enabled(s.rules.DetectLostErrorContext) && cppLostContextThrow.MatchString(line) && !strings.Contains(line, "what()") {
		s.add("reliability.lost-error-context", "warn", lineNo, "catch block replaces the original exception without preserving diagnostic context", "medium", "error", "throw-runtime-error")
	}
	if enabled(s.rules.DetectSwallowedError) && cppSwallowedCatch.MatchString(line) {
		s.add("reliability.swallowed-error", "fail", lineNo, "catch block exits without surfacing or preserving the caught exception", "high", "error", "catch-swallowed")
	}
}

var cppRetryPatterns = retryPatterns{
	language:      "C++",
	retry:         cppRetryHint.MatchString,
	backoff:       cppBackoffHint.MatchString,
	nonIdempotent: cppNonIdempotent.MatchString,
	idempotent:    cppIdempotency.MatchString,
}

func (s *cppReliabilityScan) checkLine(lineNo int, line string, scope lineScope) {
	if enabled(s.rules.DetectMissingTimeout) && cppOutboundCall.MatchString(line) && !strings.Contains(strings.ToLower(line), "timeout") && !strings.Contains(strings.ToLower(line), "deadline") {
		s.add("reliability.missing-timeout", "fail", lineNo, "outbound C++ dependency call has no visible timeout or deadline", "medium", "call", "dependency-without-timeout")
	}
	cppRetryPatterns.check(s.rules, lineNo, line, scope, s.add)
	if cppThreadLaunch.MatchString(line) {
		s.checkThreadLaunch(lineNo, scope)
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

// checkThreadLaunch records a thread/task launch and reports unbounded or
// uncancellable launches.
func (s *cppReliabilityScan) checkThreadLaunch(lineNo int, scope lineScope) {
	if enabled(s.rules.DetectUnboundedWork) && scope.inLoop && !s.limited {
		s.add("reliability.unbounded-work", "warn", lineNo, "C++ thread/task is launched inside a loop without a visible concurrency bound", "high", "work", "thread-in-loop")
	}
	s.threadLines = append(s.threadLines, lineNo)
	if enabled(s.rules.DetectMissingCancellation) && !s.cancellable {
		s.add("reliability.missing-cancellation", "warn", lineNo, "C++ async work starts without visible stop_token, deadline, or cancellation propagation", "medium", "context", "cpp-async-work")
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
