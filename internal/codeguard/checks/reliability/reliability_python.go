package reliability

import (
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	pyHTTPCall               = regexp.MustCompile(`\b(?:requests|httpx)\.(?:get|post|put|patch|delete|head)\s*\(`)
	pyAioHTTPCall            = regexp.MustCompile(`\b(?:session|client)\.(?:get|post|put|patch|delete|head)\s*\(`)
	pyRetryLoop              = regexp.MustCompile(`^\s*(?:while\s+True\s*:|for\s+\w+\s+in\s+range\s*\()`)
	pyBackoffHint            = regexp.MustCompile(`\b(?:sleep|backoff|jitter|wait_random|wait_exponential)\b`)
	pyTaskCreate             = regexp.MustCompile(`\basyncio\.(?:create_task|ensure_future)\s*\(`)
	pyConcurrencyLimit       = regexp.MustCompile(`\b(?:Semaphore|BoundedSemaphore|TaskGroup|CapacityLimiter)\s*\(`)
	pyCancellationHint       = regexp.MustCompile(`(?i)\b(?:cancel|cancelled|CancelledError|timeout|signal|lifespan|shutdown|request\.is_disconnected)\b`)
	pyServerStart            = regexp.MustCompile(`\b(?:uvicorn\.run|web\.run_app|app\.run|serve_forever|run_forever|start_server)\s*\(`)
	pyShutdownHint           = regexp.MustCompile(`(?i)\b(?:SIGTERM|SIGINT|signal\.|add_signal_handler|shutdown|lifespan|cleanup_ctx|on_shutdown|graceful)\b`)
	pySwallowedExcept        = regexp.MustCompile(`^\s*(?:pass|return\s+None|return\s*$|continue)\s*(?:#.*)?$`)
	pyRecoverableRaise       = regexp.MustCompile(`\braise\s+(?:RuntimeError|Exception)\s*\(`)
	pyLostContextRaise       = regexp.MustCompile(`^\s*raise\s+(?:RuntimeError|Exception|ValueError)\s*\(`)
	pyCloseCall              = regexp.MustCompile(`\.(?:close|aclose)\s*\(`)
	pyOpenCall               = regexp.MustCompile(`\bopen\s*\(`)
	pyIdempotencyHint        = regexp.MustCompile(`(?i)idempot|dedupe|dedup|processed|message_id|event_id`)
	pyNonIdempotentRetryCall = regexp.MustCompile(`(?i)\b(?:post|put|patch|delete|create|update|save|insert|publish|send|charge|write)\w*\s*\(`)
)

func pythonFindingsForFile(env support.Context, file string, data []byte) []core.Finding {
	source := strings.ReplaceAll(string(data), "\r\n", "\n")
	scan := &pythonReliabilityScan{
		env:         env,
		file:        file,
		rules:       env.Config.Checks.ReliabilityRules,
		limited:     pyConcurrencyLimit.MatchString(source),
		cancellable: pyCancellationHint.MatchString(source),
		hasShutdown: pyShutdownHint.MatchString(source),
	}
	for idx, line := range strings.Split(source, "\n") {
		scan.consumeLine(idx+1, line)
	}
	scan.finish()
	scan.findings = append(scan.findings, partialFailureHiddenFindings(env, file, data)...)
	return scan.findings
}

type pythonReliabilityScan struct {
	env            support.Context
	file           string
	rules          core.ReliabilityRulesConfig
	limited        bool
	cancellable    bool
	hasShutdown    bool
	loops          []pythonLoopRegion
	excepts        []int
	openLine       int
	openLineClosed bool
	taskLines      []int
	findings       []core.Finding
}

func (s *pythonReliabilityScan) consumeLine(lineNo int, line string) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return
	}
	indent := leadingIndentWidth(line)
	s.loops = popPythonLoopRegions(s.loops, indent)
	s.excepts = popIndentedRegions(s.excepts, indent)
	inLoop := len(s.loops) > 0
	inUnboundedLoop := inUnboundedPythonLoop(s.loops)
	inExcept := len(s.excepts) > 0
	if pyRetryLoop.MatchString(line) {
		s.loops = append(s.loops, pythonLoopRegion{indent: indent, unbounded: strings.HasPrefix(trimmed, "while True")})
		inLoop = true
		inUnboundedLoop = strings.HasPrefix(trimmed, "while True") || inUnboundedLoop
	}
	if strings.HasPrefix(trimmed, "except ") || strings.HasPrefix(trimmed, "except:") {
		s.excepts = append(s.excepts, indent)
		inExcept = true
	}
	s.checkLine(lineNo, line, trimmed, inLoop, inUnboundedLoop, inExcept)
}

func (s *pythonReliabilityScan) checkLine(lineNo int, line string, trimmed string, inLoop bool, inUnboundedLoop bool, inExcept bool) {
	if enabled(s.rules.DetectMissingTimeout) && (pyHTTPCall.MatchString(line) || pyAioHTTPCall.MatchString(line)) && !strings.Contains(line, "timeout=") {
		s.add("reliability.missing-timeout", "fail", lineNo, "outbound Python HTTP call has no timeout argument", "high", "call", "http-without-timeout")
	}
	if enabled(s.rules.DetectRetryWithoutBackoff) && inLoop && looksRetryish(line) && !pyBackoffHint.MatchString(line) {
		s.add("reliability.retry-without-backoff", "warn", lineNo, "retry-like Python loop has no visible backoff or jitter", "medium", "retry", "no-backoff")
	}
	if enabled(s.rules.DetectUnboundedRetry) && inUnboundedLoop && looksRetryish(line) {
		s.add("reliability.unbounded-retry", "fail", lineNo, "retry-like Python loop can run forever without an attempt limit", "medium", "retry", "while-true")
	}
	if enabled(s.rules.DetectNonIdempotentRetry) && inLoop && pyNonIdempotentRetryCall.MatchString(line) && !pyIdempotencyHint.MatchString(line) {
		s.add("reliability.non-idempotent-retry", "fail", lineNo, "retry-like Python loop wraps a non-idempotent side effect without idempotency evidence", "medium", "retry", "side-effect")
	}
	if enabled(s.rules.DetectUnboundedWork) && pyTaskCreate.MatchString(line) && !s.limited {
		detail := "asyncio-task"
		message := "asyncio task is created without an obvious concurrency bound"
		if inLoop {
			detail = "asyncio-task-in-loop"
			message = "asyncio task is created inside a loop without an obvious concurrency bound"
		}
		s.add("reliability.unbounded-work", "warn", lineNo, message, "high", "work", detail)
	}
	if pyTaskCreate.MatchString(line) {
		s.taskLines = append(s.taskLines, lineNo)
		if enabled(s.rules.DetectMissingCancellation) && !s.cancellable {
			s.add("reliability.missing-cancellation", "warn", lineNo, "asyncio task is detached without visible cancellation, timeout, or shutdown propagation", "medium", "context", "detached-asyncio-task")
		}
	}
	if enabled(s.rules.DetectMissingGracefulShutdown) && pyServerStart.MatchString(line) && !s.hasShutdown {
		s.add("reliability.missing-graceful-shutdown", "warn", lineNo, "Python server or event loop starts without visible signal handling or graceful shutdown", "medium", "server", "python-server-start")
	}
	if enabled(s.rules.DetectSwallowedError) && inExcept && pySwallowedExcept.MatchString(trimmed) {
		s.add("reliability.swallowed-error", "fail", lineNo, "exception handler swallows the error without reporting or returning it", "high", "error", "except-swallowed")
	}
	if enabled(s.rules.DetectLostErrorContext) && inExcept && pyLostContextRaise.MatchString(trimmed) && !strings.Contains(trimmed, " from ") {
		s.add("reliability.lost-error-context", "warn", lineNo, "exception handler replaces the original exception without chaining it with 'from'", "medium", "error", "raise-without-cause")
	}
	if enabled(s.rules.DetectRecoverablePanic) && pyRecoverableRaise.MatchString(line) {
		s.add("reliability.recoverable-panic", "fail", lineNo, "production code raises a generic exception for a recoverable failure path", "medium", "exception", "generic-raise")
	}
	if enabled(s.rules.DetectResourceLeak) {
		s.trackResourceLeak(lineNo, line)
	}
}

func (s *pythonReliabilityScan) finish() {
	limit := s.rules.MaxInlineGoroutinesPerFunction
	if limit <= 0 {
		limit = 4
	}
	if enabled(s.rules.DetectMissingConcurrencyLimit) && !s.limited && len(s.taskLines) > limit {
		s.add("reliability.missing-concurrency-limit", "warn", s.taskLines[0], "file creates multiple asyncio tasks without an obvious concurrency limit", "medium", "tasks", "asyncio-create-task")
	}
}

type pythonLoopRegion struct {
	indent    int
	unbounded bool
}

func (s *pythonReliabilityScan) trackResourceLeak(lineNo int, line string) {
	if strings.Contains(line, "with ") {
		return
	}
	if pyOpenCall.MatchString(line) {
		s.openLine = lineNo
		s.openLineClosed = pyCloseCall.MatchString(line)
		return
	}
	if s.openLine > 0 && pyCloseCall.MatchString(line) {
		s.openLineClosed = true
	}
	if s.openLine > 0 && lineNo > s.openLine+5 && !s.openLineClosed {
		s.add("reliability.resource-leak", "fail", s.openLine, "opened Python resource is not closed near the acquisition path", "medium", "resource", "python-open")
		s.openLine = 0
	}
}

func (s *pythonReliabilityScan) add(ruleID string, level string, lineNo int, message string, confidence string, metaKey string, metaValue string) {
	s.findings = append(s.findings, newFinding(s.env, ruleID, level, s.file, lineNo, 1, message, confidence, metaKey, metaValue))
}

func looksRetryish(line string) bool {
	lower := strings.ToLower(line)
	return strings.Contains(lower, "retry") || strings.Contains(lower, "attempt") || strings.Contains(lower, "transient")
}

func leadingIndentWidth(line string) int {
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

func popIndentedRegions(regions []int, indent int) []int {
	for len(regions) > 0 && indent <= regions[len(regions)-1] {
		regions = regions[:len(regions)-1]
	}
	return regions
}

func popPythonLoopRegions(regions []pythonLoopRegion, indent int) []pythonLoopRegion {
	for len(regions) > 0 && indent <= regions[len(regions)-1].indent {
		regions = regions[:len(regions)-1]
	}
	return regions
}

func inUnboundedPythonLoop(regions []pythonLoopRegion) bool {
	for _, region := range regions {
		if region.unbounded {
			return true
		}
	}
	return false
}
