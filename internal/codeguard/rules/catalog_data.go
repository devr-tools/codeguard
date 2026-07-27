package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

var dataCatalog = map[string]core.RuleMetadata{
	"data.read-modify-write-race":       dataRule("data.read-modify-write-race", "fail", "Read-modify-write race", "Fails when code reads state and writes derived state without a transaction, lock, or compare-and-swap boundary.", "Move the read and write into one transaction or use an atomic conditional update."),
	"data.missing-transaction-boundary": dataRule("data.missing-transaction-boundary", "fail", "Missing transaction boundary", "Fails when related persistence writes can commit independently without a transaction boundary.", "Wrap related writes in a transaction and define rollback behavior."),
	"data.side-effect-in-transaction":   dataRule("data.side-effect-in-transaction", "fail", "External side effect in transaction", "Fails when external side effects are performed inside retried or rollback-capable transactions.", "Move external side effects after commit, or persist an outbox record inside the transaction."),
	"data.non-idempotent-consumer":      dataRule("data.non-idempotent-consumer", "fail", "Non-idempotent consumer", "Fails when a message/event consumer performs side effects without idempotency evidence.", "Use a deduplication key, inbox table, processed-message guard, or idempotent operation."),
	"data.missing-deduplication":        dataRule("data.missing-deduplication", "warn", "Missing deduplication", "Warns when message or event handling lacks deduplication evidence.", "Record and check a stable message, event, or idempotency key before side effects."),
	"data.unsafe-dual-write":            dataRule("data.unsafe-dual-write", "fail", "Unsafe dual write", "Fails when code writes to two systems without a strategy for partial failure.", "Use a transaction plus outbox, saga, reconciliation job, or another explicit consistency strategy."),
	"data.missing-outbox-strategy":      dataRule("data.missing-outbox-strategy", "fail", "Missing outbox strategy", "Fails when code persists state and publishes an event without an outbox or equivalent strategy.", "Persist an outbox record in the same transaction and publish asynchronously after commit."),
	"data.unstable-pagination":          dataRule("data.unstable-pagination", "warn", "Unstable pagination", "Warns when paginated reads lack deterministic ordering or cursor stability.", "Add deterministic ordering and prefer cursor/keyset pagination for changing datasets."),
	"data.unbounded-read":               dataRule("data.unbounded-read", "warn", "Unbounded database read", "Warns when database reads lack a limit, cursor, stream, or bounded filter.", "Add a limit, cursor, stream, or explicit bounded query policy."),
	"data.exactly-once-assumption":      dataRule("data.exactly-once-assumption", "warn", "Exactly-once delivery assumption", "Warns when code assumes exactly-once message delivery without idempotency or deduplication evidence.", "Treat delivery as at-least-once and make handlers idempotent."),
	"data.cache-without-policy":         dataRule("data.cache-without-policy", "warn", "Cache without policy", "Warns when production caches lack TTL, invalidation, or ownership policy evidence.", "Define TTL, invalidation ownership, and stale-data behavior."),
}

func dataRule(id string, level string, title string, description string, howToFix string) core.RuleMetadata {
	return core.RuleMetadata{
		ID:             id,
		Section:        "Data Correctness",
		DefaultLevel:   level,
		ExecutionModel: core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.FixedRuleLanguageCoverage(
			core.RuleLanguageGo,
			core.RuleLanguageTypeScript,
			core.RuleLanguageJavaScript,
			core.RuleLanguagePython,
		),
		Title:       title,
		Description: description,
		HowToFix:    howToFix,
	}
}
