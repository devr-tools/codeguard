package data

import (
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	cppDataRead     = regexp.MustCompile(`(?i)\b(?:select|query|execute|find|load)\w*\s*\(`)
	cppDataLimit    = regexp.MustCompile(`(?i)\blimit\s*\(|\bwhere\s*\(|cursor|stream`)
	cppDataOffset   = regexp.MustCompile(`(?i)\boffset\s*\(`)
	cppDataOrder    = regexp.MustCompile(`(?i)\border_by\s*\(|order\s+by`)
	cppDataWrite    = regexp.MustCompile(`(?i)\b(?:create|update|delete|save|insert|upsert|exec)\w*\s*\(`)
	cppDataPublish  = regexp.MustCompile(`(?i)\b(?:publish|emit|send|enqueue|dispatch)\w*\s*\(`)
	cppDataTx       = regexp.MustCompile(`(?i)transaction|begin_tx|with_tx|txn`)
	cppDataOutbox   = regexp.MustCompile(`(?i)outbox`)
	cppDataDedupe   = regexp.MustCompile(`(?i)idempot|dedupe|dedup|processed|message_id|event_id`)
	cppDataConsumer = regexp.MustCompile(`(?i)\b(?:Handle|Consume|Process|OnMessage|OnEvent)\w*\s*\(`)
	cppDataCacheSet = regexp.MustCompile(`(?i)\bcache\.(?:set|put)\s*\(`)
	cppDataTTL      = regexp.MustCompile(`(?i)ttl|expire|expires`)
)

func cppFindingsForFile(env support.Context, file string, data []byte) []core.Finding {
	source := strings.ReplaceAll(string(data), "\r\n", "\n")
	masked := support.MaskCLikeSource(source, support.CLikeCPP)
	scan := &cppDataScan{env: env, file: file, rules: env.Config.Checks.DataRules}
	for idx, line := range strings.Split(masked, "\n") {
		scan.consumeLine(idx+1, line)
	}
	scan.consumeRawSource(source)
	scan.finish()
	return scan.findings
}

type cppDataScan struct {
	env   support.Context
	file  string
	rules core.DataRulesConfig
	dataFlowEvidence
	findings []core.Finding
}

var cppDataPatterns = dataLinePatterns{
	transaction: cppDataTx,
	outbox:      cppDataOutbox,
	dedupe:      cppDataDedupe,
	write:       cppDataWrite,
	read:        cppDataRead,
	publish:     cppDataPublish,
	consumer:    cppDataConsumer,
}

func (s *cppDataScan) consumeLine(lineNo int, line string) {
	s.record(lineNo, line, cppDataPatterns)
	if enabled(s.rules.DetectUnstablePagination) && cppDataOffset.MatchString(line) && !cppDataOrder.MatchString(line) {
		s.add("data.unstable-pagination", "warn", lineNo, "C++ query uses offset pagination without deterministic ordering", "high", "query", "offset-without-order")
	}
	if enabled(s.rules.DetectUnboundedRead) && cppDataRead.MatchString(line) && !cppDataLimit.MatchString(line) {
		s.add("data.unbounded-read", "warn", lineNo, "C++ database read has no visible limit, stream, cursor, or bounded filter", "medium", "query", "unbounded-read")
	}
	if enabled(s.rules.DetectCacheWithoutPolicy) && cppDataCacheSet.MatchString(line) && !cppDataTTL.MatchString(line) {
		s.add("data.cache-without-policy", "warn", lineNo, "C++ cache write lacks TTL or expiration policy evidence", "medium", "cache", "set-without-ttl")
	}
}

func (s *cppDataScan) consumeRawSource(source string) {
	if !enabled(s.rules.DetectExactlyOnceAssumption) {
		return
	}
	for idx, line := range strings.Split(source, "\n") {
		if strings.Contains(strings.ToLower(line), "exactly once") && !cppDataDedupe.MatchString(line) {
			s.add("data.exactly-once-assumption", "warn", idx+1, "C++ code assumes exactly-once delivery without idempotency evidence", "low", "comment", "exactly-once")
		}
	}
}

func (s *cppDataScan) finish() {
	s.fileFindings(s.rules, "C++", s.add)
}

func (s *cppDataScan) add(ruleID string, level string, lineNo int, message string, confidence string, metaKey string, metaValue string) {
	s.findings = append(s.findings, newFinding(s.env, ruleID, level, s.file, lineNo, 1, message, confidence, metaKey, metaValue))
}
