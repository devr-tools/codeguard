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
	tsRetryHint         = regexp.MustCompile(`(?i)retry|attempt|transient`)
	tsBackoffHint       = regexp.MustCompile(`(?i)backoff|jitter|setTimeout|sleep|delay`)
	tsPromiseInLoop     = regexp.MustCompile(`\b(?:new\s+Promise|fetch|axios\.|Promise\.all|[A-Za-z_$][\w$]*Async|fetch[A-Za-z_$][\w$]*)\s*\(`)
	tsLimitHint         = regexp.MustCompile(`p-limit|pLimit|Bottleneck|PQueue|Semaphore|AbortSignal|AbortController`)
	tsSwallowedCatch    = regexp.MustCompile(`catch\s*\([^)]*\)\s*\{\s*(?:return\s+undefined\s*;?|return\s*;?|console\.(?:log|warn|error)\([^)]*\)\s*;?)?\s*\}`)
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
		env:     env,
		file:    file,
		rules:   env.Config.Checks.ReliabilityRules,
		limited: tsLimitHint.MatchString(source),
	}
	for idx, line := range strings.Split(code, "\n") {
		scan.consumeLine(idx+1, line)
	}
	if enabled(scan.rules.DetectSwallowedError) {
		for idx, rawLine := range strings.Split(source, "\n") {
			if tsSwallowedCatch.MatchString(rawLine) {
				scan.add("reliability.swallowed-error", "fail", idx+1, "catch block swallows an error without returning or propagating it", "high", "error", "catch-swallowed")
			}
		}
	}
	return scan.findings
}

type tsReliabilityScan struct {
	env      support.Context
	file     string
	rules    core.ReliabilityRulesConfig
	limited  bool
	depth    int
	loops    []int
	findings []core.Finding
}

func (s *tsReliabilityScan) consumeLine(lineNo int, line string) {
	startsLoop := tsLoopStart.MatchString(line)
	inLoop := len(s.loops) > 0 || startsLoop
	s.checkLine(lineNo, line, inLoop)
	next := s.depth + strings.Count(line, "{") - strings.Count(line, "}")
	if startsLoop && next > s.depth {
		s.loops = append(s.loops, s.depth)
	}
	for len(s.loops) > 0 && next <= s.loops[len(s.loops)-1] {
		s.loops = s.loops[:len(s.loops)-1]
	}
	s.depth = next
}

func (s *tsReliabilityScan) checkLine(lineNo int, line string, inLoop bool) {
	if enabled(s.rules.DetectMissingTimeout) && (tsFetchCall.MatchString(line) || tsAxiosCall.MatchString(line)) && !strings.Contains(line, "timeout") && !strings.Contains(line, "signal") {
		s.add("reliability.missing-timeout", "fail", lineNo, "outbound TypeScript/JavaScript HTTP call lacks timeout or abort signal evidence", "medium", "call", "http-without-timeout")
	}
	if enabled(s.rules.DetectUnboundedWork) && inLoop && !s.limited && tsPromiseInLoop.MatchString(line) {
		s.add("reliability.unbounded-work", "warn", lineNo, "promise or HTTP work starts inside a loop without a visible concurrency limit", "medium", "work", "promise-in-loop")
	}
	if enabled(s.rules.DetectRetryWithoutBackoff) && inLoop && tsRetryHint.MatchString(line) && !tsBackoffHint.MatchString(line) {
		s.add("reliability.retry-without-backoff", "warn", lineNo, "retry-like JavaScript loop has no visible backoff or jitter", "medium", "retry", "no-backoff")
	}
	if enabled(s.rules.DetectNonIdempotentRetry) && inLoop && tsNonIdempotentCall.MatchString(line) && !tsIdempotencyHint.MatchString(line) {
		s.add("reliability.non-idempotent-retry", "fail", lineNo, "retry-like JavaScript loop wraps a non-idempotent side effect without idempotency evidence", "medium", "retry", "side-effect")
	}
	if enabled(s.rules.DetectRecoverablePanic) && tsGenericThrow.MatchString(line) {
		s.add("reliability.recoverable-panic", "fail", lineNo, "production code throws a generic exception for a recoverable failure path", "medium", "exception", "generic-throw")
	}
}

func (s *tsReliabilityScan) add(ruleID string, level string, lineNo int, message string, confidence string, metaKey string, metaValue string) {
	s.findings = append(s.findings, newFinding(s.env, ruleID, level, s.file, lineNo, 1, message, confidence, metaKey, metaValue))
}
