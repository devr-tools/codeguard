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
	tsDataSingleBound = regexp.MustCompile(`(?i)\.(?:single|maybesingle)\s*\(`)
	tsDataEqualityKey = regexp.MustCompile(`(?i)\.(?:eq|match)\s*\(\s*["'](?:id|.*_id|.*id)["']`)
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
	scan.consumeRawSource(source)
	scan.finish()
	return scan.findings
}

type tsDataScan struct {
	env   support.Context
	file  string
	rules core.DataRulesConfig
	dataFlowEvidence
	findings []core.Finding
}

var typeScriptDataPatterns = dataLinePatterns{
	transaction: tsDataTx,
	outbox:      tsDataOutbox,
	dedupe:      tsDataDedupe,
	write:       tsDataWrite,
	read:        tsDataRead,
	publish:     tsDataPublish,
	consumer:    tsDataConsumer,
}

func (s *tsDataScan) consumeLine(lineNo int, line string) {
	s.record(lineNo, line, typeScriptDataPatterns)
	if enabled(s.rules.DetectUnstablePagination) && tsDataLimitOffset.MatchString(line) && !tsDataOrder.MatchString(line) {
		s.add("data.unstable-pagination", "warn", lineNo, "TypeScript/JavaScript query uses offset pagination without deterministic ordering", "high", "query", "offset-without-order")
	}
	if enabled(s.rules.DetectUnboundedRead) && tsDataRead.MatchString(line) && !tsDataReadIsBounded(line) {
		s.add("data.unbounded-read", "warn", lineNo, "TypeScript/JavaScript database read has no visible limit or cursor bound", "medium", "query", "unbounded-read")
	}
	if enabled(s.rules.DetectCacheWithoutPolicy) && tsDataCacheSet.MatchString(line) && !tsDataTTL.MatchString(line) {
		s.add("data.cache-without-policy", "warn", lineNo, "TypeScript/JavaScript cache write lacks TTL or expiration policy evidence", "medium", "cache", "set-without-ttl")
	}
}

func tsDataReadIsBounded(line string) bool {
	if tsDataLimit.MatchString(line) || tsDataSingleBound.MatchString(line) {
		return true
	}
	return tsDataWrite.MatchString(line) && tsDataEqualityKey.MatchString(line)
}

func (s *tsDataScan) consumeRawSource(source string) {
	if !enabled(s.rules.DetectExactlyOnceAssumption) {
		return
	}
	for idx, line := range strings.Split(source, "\n") {
		if tsExactlyOnce.MatchString(line) && !tsDataDedupe.MatchString(line) {
			s.add("data.exactly-once-assumption", "warn", idx+1, "TypeScript/JavaScript code assumes exactly-once delivery without idempotency evidence", "low", "comment", "exactly-once")
		}
	}
}

func (s *tsDataScan) finish() {
	s.fileFindings(s.rules, "TypeScript/JavaScript", s.add)
}

func (s *tsDataScan) add(ruleID string, level string, lineNo int, message string, confidence string, metaKey string, metaValue string) {
	s.findings = append(s.findings, newFinding(s.env, ruleID, level, s.file, lineNo, 1, message, confidence, metaKey, metaValue))
}
