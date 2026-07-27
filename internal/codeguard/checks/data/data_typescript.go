package data

import (
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	tsDataRead        = regexp.MustCompile(`(?i)\.(?:findMany|findFirst|findUnique|query|execute|select)\s*\(`)
	tsDataLimitOffset = regexp.MustCompile(`(?i)\b(?:skip|offset)\s*:`)
	tsDataOrder       = regexp.MustCompile(`(?i)\borderBy\s*:|order\s+by`)
	tsDataLimit       = regexp.MustCompile(`(?i)\b(?:take|limit)\s*:|limit\s+\d+|cursor\s*:`)
	tsDataWrite       = regexp.MustCompile(`(?i)\.(?:create|update|delete|upsert|insert|save)\s*\(`)
	tsDataPublish     = regexp.MustCompile(`(?i)\.(?:publish|emit|send|enqueue|dispatch)\s*\(`)
	tsDataTx          = regexp.MustCompile(`(?i)transaction|\$transaction|withTransaction`)
	tsDataOutbox      = regexp.MustCompile(`(?i)outbox`)
	tsDataDedupe      = regexp.MustCompile(`(?i)idempot|dedupe|dedup|processed|messageId|eventId`)
	tsDataConsumer    = regexp.MustCompile(`(?i)\b(?:handle|consume|process|onMessage|onEvent)\w*\s*\(`)
	tsDataCacheSet    = regexp.MustCompile(`(?i)\bcache\.(?:set|put)\s*\(`)
	tsDataTTL         = regexp.MustCompile(`(?i)ttl|expires|expire|maxAge`)
	tsExactlyOnce     = regexp.MustCompile(`(?i)exactly once`)
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
	scan := &tsDataScan{env: env, file: file, rules: env.Config.Checks.DataRules}
	for idx, line := range strings.Split(code, "\n") {
		scan.consumeLine(idx+1, line)
	}
	scan.finish()
	return scan.findings
}

type tsDataScan struct {
	env          support.Context
	file         string
	rules        core.DataRulesConfig
	writeLines   []int
	publishLines []int
	consumerLine int
	hasTx        bool
	hasOutbox    bool
	hasDedupe    bool
	findings     []core.Finding
}

func (s *tsDataScan) consumeLine(lineNo int, line string) {
	if tsDataTx.MatchString(line) {
		s.hasTx = true
	}
	if tsDataOutbox.MatchString(line) {
		s.hasOutbox = true
	}
	if tsDataDedupe.MatchString(line) {
		s.hasDedupe = true
	}
	if tsDataWrite.MatchString(line) {
		s.writeLines = append(s.writeLines, lineNo)
	}
	if tsDataPublish.MatchString(line) {
		s.publishLines = append(s.publishLines, lineNo)
	}
	if enabled(s.rules.DetectUnstablePagination) && tsDataLimitOffset.MatchString(line) && !tsDataOrder.MatchString(line) {
		s.add("data.unstable-pagination", "warn", lineNo, "TypeScript/JavaScript query uses offset pagination without deterministic ordering", "high", "query", "offset-without-order")
	}
	if enabled(s.rules.DetectUnboundedRead) && tsDataRead.MatchString(line) && !tsDataLimit.MatchString(line) {
		s.add("data.unbounded-read", "warn", lineNo, "TypeScript/JavaScript database read has no visible limit or cursor bound", "medium", "query", "unbounded-read")
	}
	if tsDataConsumer.MatchString(line) {
		s.consumerLine = lineNo
	}
	if enabled(s.rules.DetectCacheWithoutPolicy) && tsDataCacheSet.MatchString(line) && !tsDataTTL.MatchString(line) {
		s.add("data.cache-without-policy", "warn", lineNo, "TypeScript/JavaScript cache write lacks TTL or expiration policy evidence", "medium", "cache", "set-without-ttl")
	}
	if enabled(s.rules.DetectExactlyOnceAssumption) && tsExactlyOnce.MatchString(line) && !tsDataDedupe.MatchString(line) {
		s.add("data.exactly-once-assumption", "warn", lineNo, "TypeScript/JavaScript code assumes exactly-once delivery without idempotency evidence", "low", "comment", "exactly-once")
	}
}

func (s *tsDataScan) finish() {
	if len(s.writeLines) > s.rules.MaxWritesWithoutTransaction && !s.hasTx {
		s.add("data.missing-transaction-boundary", "fail", s.writeLines[0], "TypeScript/JavaScript code performs multiple persistence writes without transaction evidence", "medium", "writes", "multiple")
	}
	if len(s.writeLines) > 0 && len(s.publishLines) > 0 && !s.hasOutbox {
		if enabled(s.rules.DetectUnsafeDualWrite) {
			s.add("data.unsafe-dual-write", "fail", s.writeLines[0], "TypeScript/JavaScript code writes state and publishes/sends work without a consistency strategy", "medium", "pattern", "write-plus-side-effect")
		}
		if enabled(s.rules.DetectMissingOutboxStrategy) {
			s.add("data.missing-outbox-strategy", "fail", s.publishLines[0], "TypeScript/JavaScript state write is paired with publish/send without outbox evidence", "medium", "pattern", "missing-outbox")
		}
	}
	if s.consumerLine > 0 && len(s.publishLines) > 0 && !s.hasDedupe {
		if enabled(s.rules.DetectNonIdempotentConsumer) {
			s.add("data.non-idempotent-consumer", "fail", s.consumerLine, "TypeScript/JavaScript consumer performs side effects without idempotency evidence", "medium", "consumer", "handler")
		}
		if enabled(s.rules.DetectMissingDeduplication) {
			s.add("data.missing-deduplication", "warn", s.consumerLine, "TypeScript/JavaScript consumer has no visible deduplication guard", "medium", "consumer", "handler")
		}
	}
}

func (s *tsDataScan) add(ruleID string, level string, lineNo int, message string, confidence string, metaKey string, metaValue string) {
	s.findings = append(s.findings, newFinding(s.env, ruleID, level, s.file, lineNo, 1, message, confidence, metaKey, metaValue))
}
