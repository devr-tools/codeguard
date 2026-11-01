package data

import (
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	pySelectQuery     = regexp.MustCompile(`(?i)(?:select\s+\*|\.query\s*\(|\.execute\s*\()`)
	pyLimitOffset     = regexp.MustCompile(`(?i)limit\s+\S+\s+offset|\.offset\s*\(`)
	pyOrderBy         = regexp.MustCompile(`(?i)order\s+by|\.order_by\s*\(`)
	pyLimitBound      = regexp.MustCompile(`(?i)limit\s+\S+|\.limit\s*\(|yield_per\s*\(|stream\s*=`)
	pyWriteCall       = regexp.MustCompile(`(?i)\.(?:create|update|delete|save|insert|upsert|bulk_create|bulk_update)\s*\(`)
	pyPublishCall     = regexp.MustCompile(`(?i)\.(?:publish|emit|send|enqueue|delay|apply_async)\s*\(`)
	pyTransactionHint = regexp.MustCompile(`(?i)transaction|atomic|begin_nested|with_for_update`)
	pyOutboxHint      = regexp.MustCompile(`(?i)outbox`)
	pyDedupeHint      = regexp.MustCompile(`(?i)idempot|dedupe|dedup|processed|message_id|event_id`)
	pyConsumerDef     = regexp.MustCompile(`(?i)^\s*(?:async\s+)?def\s+(?:handle|consume|process|on_)\w*`)
	pyCacheSet        = regexp.MustCompile(`(?i)\bcache\.(?:set|put)\s*\(`)
	pyTTLHint         = regexp.MustCompile(`(?i)ttl|timeout|expire`)
)

func pythonFindingsForFile(env support.Context, file string, data []byte) []core.Finding {
	source := strings.ReplaceAll(string(data), "\r\n", "\n")
	scan := &pythonDataScan{
		env:   env,
		file:  file,
		rules: env.Config.Checks.DataRules,
	}
	for idx, line := range strings.Split(source, "\n") {
		scan.consumeLine(idx+1, line)
	}
	scan.finish()
	return scan.findings
}

type pythonDataScan struct {
	env   support.Context
	file  string
	rules core.DataRulesConfig
	dataFlowEvidence
	findings []core.Finding
}

var pythonDataPatterns = dataLinePatterns{
	transaction: pyTransactionHint,
	outbox:      pyOutboxHint,
	dedupe:      pyDedupeHint,
	write:       pyWriteCall,
	read:        pySelectQuery,
	publish:     pyPublishCall,
	consumer:    pyConsumerDef,
}

func (s *pythonDataScan) consumeLine(lineNo int, line string) {
	lower := strings.ToLower(line)
	s.record(lineNo, line, pythonDataPatterns)
	if enabled(s.rules.DetectUnstablePagination) && pyLimitOffset.MatchString(line) && !pyOrderBy.MatchString(line) {
		s.add("data.unstable-pagination", "warn", lineNo, "Python query paginates with offset without deterministic ordering", "high", "query", "offset-without-order")
	}
	if enabled(s.rules.DetectUnboundedRead) && pySelectQuery.MatchString(line) && !pyLimitBound.MatchString(line) && !strings.Contains(lower, "where ") {
		s.add("data.unbounded-read", "warn", lineNo, "Python database read has no visible limit, stream, cursor, or bounded filter", "medium", "query", "unbounded-read")
	}
	if enabled(s.rules.DetectCacheWithoutPolicy) && pyCacheSet.MatchString(line) && !pyTTLHint.MatchString(line) {
		s.add("data.cache-without-policy", "warn", lineNo, "Python cache write lacks TTL or expiration policy evidence", "medium", "cache", "set-without-ttl")
	}
	if enabled(s.rules.DetectExactlyOnceAssumption) && strings.Contains(lower, "exactly once") && !pyDedupeHint.MatchString(line) {
		s.add("data.exactly-once-assumption", "warn", lineNo, "Python code assumes exactly-once delivery without idempotency or deduplication evidence", "low", "comment", "exactly-once")
	}
}

func (s *pythonDataScan) finish() {
	s.fileFindings(s.rules, "Python", s.add)
}

func (s *pythonDataScan) add(ruleID string, level string, lineNo int, message string, confidence string, metaKey string, metaValue string) {
	s.findings = append(s.findings, newFinding(s.env, ruleID, level, s.file, lineNo, 1, message, confidence, metaKey, metaValue))
}
