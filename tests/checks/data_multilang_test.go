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
	assertFindingRulePresent(t, report, "Data Correctness", "data.missing-outbox-strategy")
	assertFindingRulePresent(t, report, "Data Correctness", "data.unstable-pagination")
	assertFindingRulePresent(t, report, "Data Correctness", "data.non-idempotent-consumer")
	assertFindingRulePresent(t, report, "Data Correctness", "data.cache-without-policy")
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
	assertFindingRulePresent(t, report, "Data Correctness", "data.missing-outbox-strategy")
	assertFindingRulePresent(t, report, "Data Correctness", "data.unstable-pagination")
	assertFindingRulePresent(t, report, "Data Correctness", "data.non-idempotent-consumer")
	assertFindingRulePresent(t, report, "Data Correctness", "data.cache-without-policy")
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
	assertFindingRulePresent(t, report, "Data Correctness", "data.missing-outbox-strategy")
	assertFindingRulePresent(t, report, "Data Correctness", "data.unstable-pagination")
	assertFindingRulePresent(t, report, "Data Correctness", "data.non-idempotent-consumer")
	assertFindingRulePresent(t, report, "Data Correctness", "data.cache-without-policy")
}
