package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func dataLangConfig(name string, dir string, language string) codeguard.Config {
	cfg := dataConfig(name, dir)
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: language}}
	return cfg
}

func TestDataPythonDetectsTransactionPaginationAndConsumerGaps(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "service.py"), `
def save_and_publish(repo, bus, cache):
    repo.save(order)
    repo.update(order)
    bus.publish(event)
    cache.set("order", order)
    session.execute("SELECT * FROM orders LIMIT 20 OFFSET 40")

def handle_message(email, event):
    email.send(event)
`)

	report, err := codeguard.Run(context.Background(), dataLangConfig("data-python", dir, "python"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Data Correctness", "data.missing-transaction-boundary")
	assertFindingRulePresent(t, report, "Data Correctness", "data.unsafe-dual-write")
	assertFindingRulePresent(t, report, "Data Correctness", "data.missing-outbox-strategy")
	assertFindingRulePresent(t, report, "Data Correctness", "data.unstable-pagination")
	assertFindingRulePresent(t, report, "Data Correctness", "data.non-idempotent-consumer")
	assertFindingRulePresent(t, report, "Data Correctness", "data.missing-deduplication")
	assertFindingRulePresent(t, report, "Data Correctness", "data.cache-without-policy")
}

func TestDataPythonDetectsUnboundedReadAndExactlyOnceAssumption(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "queries.py"), `
def list_orders(session):
    return session.execute("SELECT * FROM orders")

# broker provides exactly once delivery
def consume(event):
    return event
`)

	report, err := codeguard.Run(context.Background(), dataLangConfig("data-python-read", dir, "python"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Data Correctness", "data.unbounded-read")
	assertFindingRulePresent(t, report, "Data Correctness", "data.exactly-once-assumption")
}

func TestDataPythonAcceptsTransactionalOutboxAndBoundedPolicies(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "safe_service.py"), `
def save_and_publish(repo, outbox, cache, session, event):
    with transaction.atomic():
        repo.save(order)
        repo.update(order)
        outbox.publish(event)
    cache.set("order", order, timeout=60)
    session.execute("SELECT * FROM orders WHERE account_id = :id ORDER BY id LIMIT 20")

# exactly once is handled by idempotency and dedupe keys
def handle_message(email, event):
    if processed.exists(event.message_id):
        return
    email.send(event)
`)

	report, err := codeguard.Run(context.Background(), dataLangConfig("data-python-safe", dir, "python"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Data Correctness", "data.missing-transaction-boundary")
	assertFindingRuleAbsent(t, report, "Data Correctness", "data.unsafe-dual-write")
	assertFindingRuleAbsent(t, report, "Data Correctness", "data.missing-outbox-strategy")
	assertFindingRuleAbsent(t, report, "Data Correctness", "data.unstable-pagination")
	assertFindingRuleAbsent(t, report, "Data Correctness", "data.unbounded-read")
	assertFindingRuleAbsent(t, report, "Data Correctness", "data.non-idempotent-consumer")
	assertFindingRuleAbsent(t, report, "Data Correctness", "data.missing-deduplication")
	assertFindingRuleAbsent(t, report, "Data Correctness", "data.cache-without-policy")
	assertFindingRuleAbsent(t, report, "Data Correctness", "data.exactly-once-assumption")
}

func TestDataTypeScriptDetectsTransactionPaginationAndConsumerGaps(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "service.ts"), `
async function saveAndPublish(db, bus, cache) {
  await db.order.create({});
  await db.order.update({});
  await bus.publish(event);
  cache.set("order", order);
  await db.order.findMany({ skip: 20 });
}

async function handleMessage(email, event) {
  await email.send(event);
}
`)

	report, err := codeguard.Run(context.Background(), dataLangConfig("data-ts", dir, "typescript"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Data Correctness", "data.missing-transaction-boundary")
	assertFindingRulePresent(t, report, "Data Correctness", "data.unsafe-dual-write")
	assertFindingRulePresent(t, report, "Data Correctness", "data.missing-outbox-strategy")
	assertFindingRulePresent(t, report, "Data Correctness", "data.unstable-pagination")
	assertFindingRulePresent(t, report, "Data Correctness", "data.unbounded-read")
	assertFindingRulePresent(t, report, "Data Correctness", "data.non-idempotent-consumer")
	assertFindingRulePresent(t, report, "Data Correctness", "data.missing-deduplication")
	assertFindingRulePresent(t, report, "Data Correctness", "data.cache-without-policy")
}

func TestDataTypeScriptDetectsExactlyOnceAssumption(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "consumer.ts"), `
// broker gives exactly once delivery
export function consume(event: Event) {
  return event;
}
`)

	report, err := codeguard.Run(context.Background(), dataLangConfig("data-ts-exactly-once", dir, "typescript"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Data Correctness", "data.exactly-once-assumption")
}

func TestDataTypeScriptAcceptsTransactionalOutboxAndBoundedPolicies(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "safe_service.ts"), `
async function saveAndPublish(db, outbox, cache, event) {
  await db.$transaction(async (tx) => {
    await tx.order.create({});
    await tx.order.update({});
    await outbox.publish(event);
  });
  cache.set("order", event, { ttl: 60 });
  await db.order.findMany({ where: { accountId: event.accountId }, orderBy: { id: "asc" }, take: 20 });
}

// exactly once is handled by idempotency and dedupe keys
async function handleMessage(email, event) {
  if (await processed.has(event.messageId)) return;
  await email.send(event);
}
`)

	report, err := codeguard.Run(context.Background(), dataLangConfig("data-ts-safe", dir, "typescript"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Data Correctness", "data.missing-transaction-boundary")
	assertFindingRuleAbsent(t, report, "Data Correctness", "data.unsafe-dual-write")
	assertFindingRuleAbsent(t, report, "Data Correctness", "data.missing-outbox-strategy")
	assertFindingRuleAbsent(t, report, "Data Correctness", "data.unstable-pagination")
	assertFindingRuleAbsent(t, report, "Data Correctness", "data.unbounded-read")
	assertFindingRuleAbsent(t, report, "Data Correctness", "data.non-idempotent-consumer")
	assertFindingRuleAbsent(t, report, "Data Correctness", "data.missing-deduplication")
	assertFindingRuleAbsent(t, report, "Data Correctness", "data.cache-without-policy")
	assertFindingRuleAbsent(t, report, "Data Correctness", "data.exactly-once-assumption")
}

func TestDataTypeScriptAcceptsSupabaseMaybeSingleBounds(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "override-store.ts"), `
async function markOverrideCleared(update) {
  const { data, error } = await update.select("id").maybeSingle();
  return { data, error };
}
`)

	report, err := codeguard.Run(context.Background(), dataLangConfig("data-ts-supabase-single", dir, "typescript"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Data Correctness", "data.unbounded-read")
}

func TestDataJavaScriptUsesSameDetector(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "service.js"), `
async function list(db) {
  return db.order.findMany({});
}
`)

	report, err := codeguard.Run(context.Background(), dataLangConfig("data-js", dir, "javascript"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Data Correctness", "data.unbounded-read")
}

func TestDataJavaScriptDetectsTransactionPaginationConsumerAndExactlyOnceGaps(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "service.js"), `
async function saveAndPublish(db, bus, cache) {
  await db.order.create({});
  await db.order.update({});
  await bus.publish(event);
  cache.set("order", order);
  await db.order.findMany({ skip: 20 });
}

// broker gives exactly once delivery
async function handleMessage(email, event) {
  await email.send(event);
}
`)

	report, err := codeguard.Run(context.Background(), dataLangConfig("data-js-gaps", dir, "javascript"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Data Correctness", "data.missing-transaction-boundary")
	assertFindingRulePresent(t, report, "Data Correctness", "data.unsafe-dual-write")
	assertFindingRulePresent(t, report, "Data Correctness", "data.missing-outbox-strategy")
	assertFindingRulePresent(t, report, "Data Correctness", "data.unstable-pagination")
	assertFindingRulePresent(t, report, "Data Correctness", "data.unbounded-read")
	assertFindingRulePresent(t, report, "Data Correctness", "data.non-idempotent-consumer")
	assertFindingRulePresent(t, report, "Data Correctness", "data.missing-deduplication")
	assertFindingRulePresent(t, report, "Data Correctness", "data.cache-without-policy")
	assertFindingRulePresent(t, report, "Data Correctness", "data.exactly-once-assumption")
}

func TestDataCPPDetectsTransactionPaginationAndConsumerGaps(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "service.cpp"), `
void SaveAndPublish(Repo& repo, Bus& bus, Cache& cache) {
  repo.save(order);
  repo.update(order);
  bus.publish(event);
  cache.set("order", order);
  db.offset(20);
}

void HandleMessage(Email& email, Event event) {
  email.send(event);
}
`)

	report, err := codeguard.Run(context.Background(), dataLangConfig("data-cpp", dir, "cpp"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Data Correctness", "data.missing-transaction-boundary")
	assertFindingRulePresent(t, report, "Data Correctness", "data.unsafe-dual-write")
	assertFindingRulePresent(t, report, "Data Correctness", "data.missing-outbox-strategy")
	assertFindingRulePresent(t, report, "Data Correctness", "data.unstable-pagination")
	assertFindingRulePresent(t, report, "Data Correctness", "data.non-idempotent-consumer")
	assertFindingRulePresent(t, report, "Data Correctness", "data.missing-deduplication")
	assertFindingRulePresent(t, report, "Data Correctness", "data.cache-without-policy")
}

func TestDataCPPDetectsUnboundedReadAndExactlyOnceAssumption(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "queries.cpp"), `
void ListOrders(DB& db) {
  db.query("SELECT * FROM orders");
}

// broker gives exactly once delivery
void Consume(Event event) {}
`)

	report, err := codeguard.Run(context.Background(), dataLangConfig("data-cpp-read", dir, "cpp"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Data Correctness", "data.unbounded-read")
	assertFindingRulePresent(t, report, "Data Correctness", "data.exactly-once-assumption")
}

func TestDataCPPAcceptsTransactionalOutboxAndBoundedPolicies(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "safe_service.cpp"), `
void SaveAndPublish(Repo& repo, Outbox& outbox, Cache& cache, DB& db, Event event) {
  auto txn = db.begin_tx();
  repo.save(txn, order);
  repo.update(txn, order);
  outbox.publish(event);
  cache.set("order", order, ttl);
  db.query().where(account_id).order_by(id).limit(20);
}

// exactly once is handled by idempotency and dedupe keys
void HandleMessage(Email& email, Event event) {
  if (processed.contains(event.message_id)) return;
  email.send(event);
}
`)

	report, err := codeguard.Run(context.Background(), dataLangConfig("data-cpp-safe", dir, "cpp"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Data Correctness", "data.missing-transaction-boundary")
	assertFindingRuleAbsent(t, report, "Data Correctness", "data.unsafe-dual-write")
	assertFindingRuleAbsent(t, report, "Data Correctness", "data.missing-outbox-strategy")
	assertFindingRuleAbsent(t, report, "Data Correctness", "data.unstable-pagination")
	assertFindingRuleAbsent(t, report, "Data Correctness", "data.unbounded-read")
	assertFindingRuleAbsent(t, report, "Data Correctness", "data.non-idempotent-consumer")
	assertFindingRuleAbsent(t, report, "Data Correctness", "data.missing-deduplication")
	assertFindingRuleAbsent(t, report, "Data Correctness", "data.cache-without-policy")
	assertFindingRuleAbsent(t, report, "Data Correctness", "data.exactly-once-assumption")
}

func TestDataDetectsReadModifyWriteRaceAcrossNonGoLanguages(t *testing.T) {
	cases := []struct {
		name     string
		language string
		file     string
		source   string
	}{
		{
			name:     "python",
			language: "python",
			file:     "counter.py",
			source: `
def increment(repo):
    current = repo.query("SELECT count FROM counters WHERE id = 1")
    repo.update(current + 1)
`,
		},
		{
			name:     "typescript",
			language: "typescript",
			file:     "counter.ts",
			source: `
async function increment(db) {
  const current = await db.counter.findUnique({ where: { id: 1 } });
  await db.counter.update({ data: { count: current.count + 1 } });
}
`,
		},
		{
			name:     "javascript",
			language: "javascript",
			file:     "counter.js",
			source: `
async function increment(db) {
  const current = await db.counter.findUnique({ where: { id: 1 } });
  await db.counter.update({ data: { count: current.count + 1 } });
}
`,
		},
		{
			name:     "cpp",
			language: "cpp",
			file:     "counter.cpp",
			source: `
void Increment(DB& db) {
  auto current = db.findCounter(id);
  db.updateCounter(id, current.count + 1);
}
`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, tc.file), tc.source)

			report, err := codeguard.Run(context.Background(), dataLangConfig("data-rmw-"+tc.name, dir, tc.language))
			if err != nil {
				t.Fatalf("run: %v", err)
			}

			assertFindingRulePresent(t, report, "Data Correctness", "data.read-modify-write-race")
		})
	}
}

func TestDataDetectsSideEffectInTransactionAcrossNonGoLanguages(t *testing.T) {
	cases := []struct {
		name     string
		language string
		file     string
		source   string
	}{
		{
			name:     "python",
			language: "python",
			file:     "transaction.py",
			source: `
def save(repo, bus):
    with transaction.atomic():
        repo.save(order)
        bus.publish(event)
`,
		},
		{
			name:     "typescript",
			language: "typescript",
			file:     "transaction.ts",
			source: `
async function save(db, bus) {
  await db.$transaction(async (tx) => {
    await tx.order.create({});
    await bus.publish(event);
  });
}
`,
		},
		{
			name:     "javascript",
			language: "javascript",
			file:     "transaction.js",
			source: `
async function save(db, bus) {
  await db.$transaction(async (tx) => {
    await tx.order.create({});
    await bus.publish(event);
  });
}
`,
		},
		{
			name:     "cpp",
			language: "cpp",
			file:     "transaction.cpp",
			source: `
void Save(DB& db, Bus& bus) {
  auto txn = db.begin_tx();
  repo.save(txn, order);
  bus.publish(event);
}
`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, tc.file), tc.source)

			report, err := codeguard.Run(context.Background(), dataLangConfig("data-side-effect-tx-"+tc.name, dir, tc.language))
			if err != nil {
				t.Fatalf("run: %v", err)
			}

			assertFindingRulePresent(t, report, "Data Correctness", "data.side-effect-in-transaction")
		})
	}
}
