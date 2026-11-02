package data

import (
	"regexp"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// dataLinePatterns are one language's line-level persistence signals.
type dataLinePatterns struct {
	transaction *regexp.Regexp
	outbox      *regexp.Regexp
	dedupe      *regexp.Regexp
	write       *regexp.Regexp
	read        *regexp.Regexp
	publish     *regexp.Regexp
	consumer    *regexp.Regexp
}

// dataFlowEvidence accumulates, across a file, where state is read,
// written, and published, the last consumer/handler definition, and whether
// transaction, outbox, and deduplication evidence appears anywhere.
type dataFlowEvidence struct {
	readLines    []int
	writeLines   []int
	publishLines []int
	consumerLine int
	hasTx        bool
	hasOutbox    bool
	hasDedupe    bool
}

func (e *dataFlowEvidence) record(lineNo int, line string, patterns dataLinePatterns) {
	e.hasTx = e.hasTx || patterns.transaction.MatchString(line)
	e.hasOutbox = e.hasOutbox || patterns.outbox.MatchString(line)
	e.hasDedupe = e.hasDedupe || patterns.dedupe.MatchString(line)
	if patterns.write.MatchString(line) {
		e.writeLines = append(e.writeLines, lineNo)
	}
	if patterns.read.MatchString(line) {
		e.readLines = append(e.readLines, lineNo)
	}
	if patterns.publish.MatchString(line) {
		e.publishLines = append(e.publishLines, lineNo)
	}
	if patterns.consumer.MatchString(line) {
		e.consumerLine = lineNo
	}
}

// addFindingFunc records one finding for a line-based data scanner.
type addFindingFunc func(ruleID string, level string, lineNo int, message string, confidence string, metaKey string, metaValue string)

// fileFindings reports the file-level consistency rules. language prefixes
// each message ("Python", "TypeScript/JavaScript", "C++").
func (e *dataFlowEvidence) fileFindings(rules core.DataRulesConfig, language string, add addFindingFunc) {
	if enabled(rules.DetectReadModifyWriteRace) && len(e.readLines) > 0 && len(e.writeLines) > 0 && !e.hasTx {
		add("data.read-modify-write-race", "fail", e.readLines[0], language+" code reads state and writes derived state without transaction or atomic update evidence", "medium", "pattern", "read-modify-write")
	}
	if len(e.writeLines) > rules.MaxWritesWithoutTransaction && !e.hasTx {
		add("data.missing-transaction-boundary", "fail", e.writeLines[0], language+" code performs multiple persistence writes without transaction evidence", "medium", "writes", "multiple")
	}
	if enabled(rules.DetectSideEffectInTransaction) && e.hasTx && len(e.publishLines) > 0 && !e.hasOutbox {
		add("data.side-effect-in-transaction", "fail", e.publishLines[0], language+" transaction block performs an external side effect that may not roll back safely", "high", "transaction", "side-effect")
	}
	if len(e.writeLines) > 0 && len(e.publishLines) > 0 && !e.hasOutbox {
		e.dualWriteFindings(rules, language, add)
	}
	if e.consumerLine > 0 && len(e.publishLines) > 0 && !e.hasDedupe {
		e.consumerFindings(rules, language, add)
	}
}

func (e *dataFlowEvidence) dualWriteFindings(rules core.DataRulesConfig, language string, add addFindingFunc) {
	if enabled(rules.DetectUnsafeDualWrite) {
		add("data.unsafe-dual-write", "fail", e.writeLines[0], language+" code writes state and publishes/sends work without a consistency strategy", "medium", "pattern", "write-plus-side-effect")
	}
	if enabled(rules.DetectMissingOutboxStrategy) {
		add("data.missing-outbox-strategy", "fail", e.publishLines[0], language+" state write is paired with publish/send without outbox evidence", "medium", "pattern", "missing-outbox")
	}
}

func (e *dataFlowEvidence) consumerFindings(rules core.DataRulesConfig, language string, add addFindingFunc) {
	if enabled(rules.DetectNonIdempotentConsumer) {
		add("data.non-idempotent-consumer", "fail", e.consumerLine, language+" consumer performs side effects without idempotency evidence", "medium", "consumer", "handler")
	}
	if enabled(rules.DetectMissingDeduplication) {
		add("data.missing-deduplication", "warn", e.consumerLine, language+" consumer has no visible deduplication guard", "medium", "consumer", "handler")
	}
}
