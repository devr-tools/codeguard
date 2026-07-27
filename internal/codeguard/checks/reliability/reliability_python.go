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
	pySwallowedExcept        = regexp.MustCompile(`^\s*(?:pass|return\s+None|return\s*$|continue)\s*(?:#.*)?$`)
	pyRecoverableRaise       = regexp.MustCompile(`\braise\s+(?:RuntimeError|Exception)\s*\(`)
	pyCloseCall              = regexp.MustCompile(`\.(?:close|aclose)\s*\(`)
	pyOpenCall               = regexp.MustCompile(`\bopen\s*\(`)
	pyIdempotencyHint        = regexp.MustCompile(`(?i)idempot|dedupe|dedup|processed|message_id|event_id`)
	pyNonIdempotentRetryCall = regexp.MustCompile(`(?i)\b(?:post|put|patch|delete|create|update|save|insert|publish|send|charge|write)\w*\s*\(`)
)

func pythonFindingsForFile(env support.Context, file string, data []byte) []core.Finding {
	source := strings.ReplaceAll(string(data), "\r\n", "\n")
	scan := &pythonReliabilityScan{
		env:     env,
		file:    file,
		rules:   env.Config.Checks.ReliabilityRules,
		limited: pyConcurrencyLimit.MatchString(source),
	}
	for idx, line := range strings.Split(source, "\n") {
		scan.consumeLine(idx+1, line)
	}
	scan.findings = append(scan.findings, partialFailureHiddenFindings(env, file, data)...)
	return scan.findings
}

type pythonReliabilityScan struct {
	env            support.Context
	file           string
	rules          core.ReliabilityRulesConfig
	limited        bool
	loops          []pythonLoopRegion
	excepts        []int
	openLine       int
	openLineClosed bool
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
	if enabled(s.rules.DetectSwallowedError) && inExcept && pySwallowedExcept.MatchString(trimmed) {
		s.add("reliability.swallowed-error", "fail", lineNo, "exception handler swallows the error without reporting or returning it", "high", "error", "except-swallowed")
	}
	if enabled(s.rules.DetectRecoverablePanic) && pyRecoverableRaise.MatchString(line) {
		s.add("reliability.recoverable-panic", "fail", lineNo, "production code raises a generic exception for a recoverable failure path", "medium", "exception", "generic-raise")
	}
	if enabled(s.rules.DetectResourceLeak) {
		s.trackResourceLeak(lineNo, line)
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
