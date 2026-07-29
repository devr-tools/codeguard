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
	env          support.Context
	file         string
	rules        core.DataRulesConfig
	readLines    []int
	writeLines   []int
	publishLines []int
	consumerLine int
	hasTx        bool
	hasOutbox    bool
	hasDedupe    bool
	findings     []core.Finding
}

func (s *pythonDataScan) consumeLine(lineNo int, line string) {
	lower := strings.ToLower(line)
	if pyTransactionHint.MatchString(line) {
		s.hasTx = true
	}
	if pyOutboxHint.MatchString(line) {
		s.hasOutbox = true
	}
	if pyDedupeHint.MatchString(line) {
		s.hasDedupe = true
	}
	if pyWriteCall.MatchString(line) {
		s.writeLines = append(s.writeLines, lineNo)
	}
	if pySelectQuery.MatchString(line) {
		s.readLines = append(s.readLines, lineNo)
	}
	if pyPublishCall.MatchString(line) {
		s.publishLines = append(s.publishLines, lineNo)
	}
	if enabled(s.rules.DetectUnstablePagination) && pyLimitOffset.MatchString(line) && !pyOrderBy.MatchString(line) {
		s.add("data.unstable-pagination", "warn", lineNo, "Python query paginates with offset without deterministic ordering", "high", "query", "offset-without-order")
	}
	if enabled(s.rules.DetectUnboundedRead) && pySelectQuery.MatchString(line) && !pyLimitBound.MatchString(line) && !strings.Contains(lower, "where ") {
		s.add("data.unbounded-read", "warn", lineNo, "Python database read has no visible limit, stream, cursor, or bounded filter", "medium", "query", "unbounded-read")
	}
	if pyConsumerDef.MatchString(line) {
		s.consumerLine = lineNo
	}
	if enabled(s.rules.DetectCacheWithoutPolicy) && pyCacheSet.MatchString(line) && !pyTTLHint.MatchString(line) {
		s.add("data.cache-without-policy", "warn", lineNo, "Python cache write lacks TTL or expiration policy evidence", "medium", "cache", "set-without-ttl")
	}
	if enabled(s.rules.DetectExactlyOnceAssumption) && strings.Contains(lower, "exactly once") && !pyDedupeHint.MatchString(line) {
		s.add("data.exactly-once-assumption", "warn", lineNo, "Python code assumes exactly-once delivery without idempotency or deduplication evidence", "low", "comment", "exactly-once")
	}
}

func (s *pythonDataScan) finish() {
	if enabled(s.rules.DetectReadModifyWriteRace) && len(s.readLines) > 0 && len(s.writeLines) > 0 && !s.hasTx {
		s.add("data.read-modify-write-race", "fail", s.readLines[0], "Python code reads state and writes derived state without transaction or atomic update evidence", "medium", "pattern", "read-modify-write")
	}
	if len(s.writeLines) > s.rules.MaxWritesWithoutTransaction && !s.hasTx {
		s.add("data.missing-transaction-boundary", "fail", s.writeLines[0], "Python code performs multiple persistence writes without transaction evidence", "medium", "writes", "multiple")
	}
	if enabled(s.rules.DetectSideEffectInTransaction) && s.hasTx && len(s.publishLines) > 0 && !s.hasOutbox {
		s.add("data.side-effect-in-transaction", "fail", s.publishLines[0], "Python transaction block performs an external side effect that may not roll back safely", "high", "transaction", "side-effect")
	}
	if len(s.writeLines) > 0 && len(s.publishLines) > 0 && !s.hasOutbox {
		if enabled(s.rules.DetectUnsafeDualWrite) {
			s.add("data.unsafe-dual-write", "fail", s.writeLines[0], "Python code writes state and publishes/sends work without a consistency strategy", "medium", "pattern", "write-plus-side-effect")
		}
		if enabled(s.rules.DetectMissingOutboxStrategy) {
			s.add("data.missing-outbox-strategy", "fail", s.publishLines[0], "Python state write is paired with publish/send without outbox evidence", "medium", "pattern", "missing-outbox")
		}
	}
	if s.consumerLine > 0 && len(s.publishLines) > 0 && !s.hasDedupe {
		if enabled(s.rules.DetectNonIdempotentConsumer) {
			s.add("data.non-idempotent-consumer", "fail", s.consumerLine, "Python consumer performs side effects without idempotency evidence", "medium", "consumer", "handler")
		}
		if enabled(s.rules.DetectMissingDeduplication) {
			s.add("data.missing-deduplication", "warn", s.consumerLine, "Python consumer has no visible deduplication guard", "medium", "consumer", "handler")
		}
	}
}

func (s *pythonDataScan) add(ruleID string, level string, lineNo int, message string, confidence string, metaKey string, metaValue string) {
	s.findings = append(s.findings, newFinding(s.env, ruleID, level, s.file, lineNo, 1, message, confidence, metaKey, metaValue))
}
