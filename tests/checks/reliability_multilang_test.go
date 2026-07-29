package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func reliabilityLangConfig(name string, dir string, language string) codeguard.Config {
	cfg := reliabilityConfig(name, dir)
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: language}}
	return cfg
}

func TestReliabilityPythonDetectsTimeoutWorkAndSwallowedError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "worker.py"), `
import asyncio
import requests

def fetch(url):
    return requests.get(url)

async def run(items):
    for item in items:
        asyncio.create_task(fetch(item))

def handle():
    try:
        fetch("https://example.com")
    except Exception:
        pass
`)

	report, err := codeguard.Run(context.Background(), reliabilityLangConfig("reliability-python", dir, "python"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Reliability", "reliability.missing-timeout")
	assertFindingRulePresent(t, report, "Reliability", "reliability.unbounded-work")
	assertFindingRulePresent(t, report, "Reliability", "reliability.swallowed-error")
}

func TestReliabilityPythonDetectsRetryResourceAndGenericRaise(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "payments.py"), `
def charge_with_retry(payments, card):
    while True:
        retry_attempt = payments.charge(card)
        if retry_attempt:
            return retry_attempt

def load_widget():
    handle = open("/tmp/widget.txt")
    use(handle)
    use(handle)
    use(handle)
    use(handle)
    use(handle)
    use(handle)
    raise RuntimeError("widget failed")
`)

	report, err := codeguard.Run(context.Background(), reliabilityLangConfig("reliability-python-retry", dir, "python"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Reliability", "reliability.unbounded-retry")
	assertFindingRulePresent(t, report, "Reliability", "reliability.retry-without-backoff")
	assertFindingRulePresent(t, report, "Reliability", "reliability.non-idempotent-retry")
	assertFindingRulePresent(t, report, "Reliability", "reliability.resource-leak")
	assertFindingRulePresent(t, report, "Reliability", "reliability.recoverable-panic")
}

func TestReliabilityPythonDetectsCancellationShutdownConcurrencyAndLostContext(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "service.py"), `
import asyncio
import uvicorn

async def run_many(items):
    asyncio.create_task(process(items[0]))
    asyncio.create_task(process(items[1]))
    asyncio.create_task(process(items[2]))
    asyncio.create_task(process(items[3]))
    asyncio.create_task(process(items[4]))

def wrap_error():
    try:
        load_customer()
    except Exception as err:
        raise RuntimeError("customer load failed")

def main():
    uvicorn.run(app)
`)

	report, err := codeguard.Run(context.Background(), reliabilityLangConfig("reliability-python-parity", dir, "python"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Reliability", "reliability.missing-cancellation")
	assertFindingRulePresent(t, report, "Reliability", "reliability.missing-concurrency-limit")
	assertFindingRulePresent(t, report, "Reliability", "reliability.missing-graceful-shutdown")
	assertFindingRulePresent(t, report, "Reliability", "reliability.lost-error-context")
}

func TestReliabilityPythonAcceptsBoundedPatterns(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "safe_worker.py"), `
import asyncio
import logging
import requests

async def run(items):
    limit = asyncio.Semaphore(4)
    async def worker(item):
        async with limit:
            return requests.get(item, timeout=5)
    return [asyncio.create_task(worker(item)) for item in items]

def load():
    try:
        with open("/tmp/widget.txt") as handle:
            return handle.read()
    except Exception as err:
        logging.exception("load failed", exc_info=err)
        raise
`)

	report, err := codeguard.Run(context.Background(), reliabilityLangConfig("reliability-python-safe", dir, "python"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Reliability", "reliability.missing-timeout")
	assertFindingRuleAbsent(t, report, "Reliability", "reliability.unbounded-work")
	assertFindingRuleAbsent(t, report, "Reliability", "reliability.swallowed-error")
	assertFindingRuleAbsent(t, report, "Reliability", "reliability.resource-leak")
}

func TestReliabilityTypeScriptDetectsTimeoutWorkAndSwallowedError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "worker.ts"), `
async function fetchUser(id: string) {
  return fetch("/users/" + id);
}

async function run(ids: string[]) {
  for (const id of ids) {
    fetchUser(id);
  }
}

async function handle() {
  try {
    await fetchUser("1");
  } catch (err) { return; }
}
`)

	report, err := codeguard.Run(context.Background(), reliabilityLangConfig("reliability-ts", dir, "typescript"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Reliability", "reliability.missing-timeout")
	assertFindingRulePresent(t, report, "Reliability", "reliability.unbounded-work")
	assertFindingRulePresent(t, report, "Reliability", "reliability.swallowed-error")
}

func TestReliabilityTypeScriptDetectsRetryAndGenericThrow(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "payments.ts"), `
async function retryCharge(payments: Payments, card: Card) {
  while (true) {
    const retryAttempt = await payments.charge(card);
    if (retryAttempt) return retryAttempt;
  }
}

function failPayment() {
  throw new Error("payment failed");
}
`)

	report, err := codeguard.Run(context.Background(), reliabilityLangConfig("reliability-ts-retry", dir, "typescript"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Reliability", "reliability.unbounded-retry")
	assertFindingRulePresent(t, report, "Reliability", "reliability.retry-without-backoff")
	assertFindingRulePresent(t, report, "Reliability", "reliability.non-idempotent-retry")
	assertFindingRulePresent(t, report, "Reliability", "reliability.recoverable-panic")
}

func TestReliabilityTypeScriptDetectsCancellationShutdownResourceConcurrencyAndLostContext(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "service.ts"), `
import fs from "fs";

async function startMany() {
  fetchUser("1");
  fetchUser("2");
  fetchUser("3");
  fetchUser("4");
  fetchUser("5");
}

function wrapError() {
  try {
    risky();
  } catch (err) { throw new Error("operation failed"); }
}

function leakStream() {
  const stream = fs.createReadStream("/tmp/input.txt");
  use(stream);
  use(stream);
  use(stream);
  use(stream);
  use(stream);
  use(stream);
  use(stream);
  use(stream);
  use(stream);
}

app.listen(3000);
`)

	report, err := codeguard.Run(context.Background(), reliabilityLangConfig("reliability-ts-parity", dir, "typescript"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Reliability", "reliability.missing-cancellation")
	assertFindingRulePresent(t, report, "Reliability", "reliability.missing-concurrency-limit")
	assertFindingRulePresent(t, report, "Reliability", "reliability.missing-graceful-shutdown")
	assertFindingRulePresent(t, report, "Reliability", "reliability.resource-leak")
	assertFindingRulePresent(t, report, "Reliability", "reliability.lost-error-context")
}

func TestReliabilityTypeScriptAcceptsBoundedPatterns(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "safe_worker.ts"), `
import pLimit from "p-limit";

async function fetchUser(url: string, signal: AbortSignal) {
  return fetch(url, { signal });
}

async function run(ids: string[], signal: AbortSignal) {
  const limit = pLimit(4);
  return Promise.all(ids.map((id) => limit(() => fetchUser("/users/" + id, signal))));
}

async function handle() {
  try {
    await fetchUser("/users/1", new AbortController().signal);
  } catch (err) {
    throw err;
  }
}
`)

	report, err := codeguard.Run(context.Background(), reliabilityLangConfig("reliability-ts-safe", dir, "typescript"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Reliability", "reliability.missing-timeout")
	assertFindingRuleAbsent(t, report, "Reliability", "reliability.unbounded-work")
	assertFindingRuleAbsent(t, report, "Reliability", "reliability.swallowed-error")
}

func TestReliabilityJavaScriptUsesSameDetector(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "worker.js"), `
async function fetchUser(id) {
  return fetch("/users/" + id);
}
`)

	report, err := codeguard.Run(context.Background(), reliabilityLangConfig("reliability-js", dir, "javascript"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Reliability", "reliability.missing-timeout")
}

func TestReliabilityJavaScriptDetectsRetryAndGenericThrow(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "payments.js"), `
async function retryCharge(payments, card) {
  while (true) {
    const retryAttempt = await payments.charge(card);
    if (retryAttempt) return retryAttempt;
  }
}

function failPayment() {
  throw new Error("payment failed");
}
`)

	report, err := codeguard.Run(context.Background(), reliabilityLangConfig("reliability-js-retry", dir, "javascript"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Reliability", "reliability.unbounded-retry")
	assertFindingRulePresent(t, report, "Reliability", "reliability.retry-without-backoff")
	assertFindingRulePresent(t, report, "Reliability", "reliability.non-idempotent-retry")
	assertFindingRulePresent(t, report, "Reliability", "reliability.recoverable-panic")
}

func TestReliabilityJavaScriptDetectsCancellationShutdownResourceConcurrencyAndLostContext(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "service.js"), `
const fs = require("fs");

async function startMany() {
  fetchUser("1");
  fetchUser("2");
  fetchUser("3");
  fetchUser("4");
  fetchUser("5");
}

function wrapError() {
  try {
    risky();
  } catch (err) { throw new Error("operation failed"); }
}

function leakStream() {
  const stream = fs.createReadStream("/tmp/input.txt");
  use(stream);
  use(stream);
  use(stream);
  use(stream);
  use(stream);
  use(stream);
  use(stream);
  use(stream);
  use(stream);
}

app.listen(3000);
`)

	report, err := codeguard.Run(context.Background(), reliabilityLangConfig("reliability-js-parity", dir, "javascript"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Reliability", "reliability.missing-cancellation")
	assertFindingRulePresent(t, report, "Reliability", "reliability.missing-concurrency-limit")
	assertFindingRulePresent(t, report, "Reliability", "reliability.missing-graceful-shutdown")
	assertFindingRulePresent(t, report, "Reliability", "reliability.resource-leak")
	assertFindingRulePresent(t, report, "Reliability", "reliability.lost-error-context")
}

func TestReliabilityDetectsHiddenPartialFailuresAcrossLanguages(t *testing.T) {
	for _, tc := range hiddenPartialFailureCases() {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, tc.file), tc.source)

			report, err := codeguard.Run(context.Background(), reliabilityLangConfig("reliability-partial-"+tc.name, dir, tc.language))
			if err != nil {
				t.Fatalf("run: %v", err)
			}

			assertFindingRulePresent(t, report, "Reliability", "reliability.partial-failure-hidden")
		})
	}
}

type reliabilityLanguageCase struct {
	name     string
	language string
	file     string
	source   string
}

func hiddenPartialFailureCases() []reliabilityLanguageCase {
	return []reliabilityLanguageCase{
		{
			name:     "go",
			language: "go",
			file:     "batch.go",
			source: `package sample

import "log"

func Process(items []Item) error {
	for _, item := range items {
		if err := process(item); err != nil {
			log.Printf("item error: %v", err)
			continue
		}
	}
	return nil
}

type Item struct{}
func process(Item) error { return nil }
`,
		},
		{
			name:     "python",
			language: "python",
			file:     "batch.py",
			source: `
import logging

def process_all(items):
    for item in items:
        try:
            process(item)
        except Exception as error:
            logging.error("item failed: %s", error)
            continue
    return None
`,
		},
		{
			name:     "typescript",
			language: "typescript",
			file:     "batch.ts",
			source: `
async function processAll(items: Item[]): Promise<void> {
  for (const item of items) {
    try {
      await process(item);
    } catch (error) {
      console.error("item error", error);
      continue;
    }
  }
  return;
}
`,
		},
		{
			name:     "javascript",
			language: "javascript",
			file:     "batch.js",
			source: `
async function processAll(items) {
  for (const item of items) {
    try {
      await process(item);
    } catch (error) {
      console.warn("item error", error);
      continue;
    }
  }
  return;
}
`,
		},
		{
			name:     "cpp",
			language: "cpp",
			file:     "batch.cpp",
			source: `
#include <iostream>

bool ProcessAll(const std::vector<Item>& items) {
  for (const auto& item : items) {
    try {
      Process(item);
    } catch (const std::exception& error) {
      std::cerr << "item error: " << error.what();
      continue;
    }
  }
  return true;
}
`,
		},
	}
}

func TestReliabilityCPPDetectsUnboundedWorkAndResourceLeak(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "worker.cpp"), `
#include <thread>

void run(int n) {
  for (int i = 0; i < n; ++i) {
    std::thread([]{}).detach();
  }
  auto* item = new Widget();
  use(item);
  more();
  more();
  more();
  more();
  more();
  more();
  more();
  more();
  more();
}
`)

	report, err := codeguard.Run(context.Background(), reliabilityLangConfig("reliability-cpp", dir, "cpp"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Reliability", "reliability.unbounded-work")
	assertFindingRulePresent(t, report, "Reliability", "reliability.resource-leak")
}

func TestReliabilityCPPDetectsRetryAndGenericThrow(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "payments.cpp"), `
#include <stdexcept>

void ChargeWithRetry(Payments& payments, Card card) {
  while (true) {
    auto retry_attempt = payments.Charge(card);
    if (retry_attempt.ok()) return;
  }
}

void FailPayment() {
  throw std::runtime_error("payment failed");
}
`)

	report, err := codeguard.Run(context.Background(), reliabilityLangConfig("reliability-cpp-retry", dir, "cpp"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Reliability", "reliability.unbounded-retry")
	assertFindingRulePresent(t, report, "Reliability", "reliability.retry-without-backoff")
	assertFindingRulePresent(t, report, "Reliability", "reliability.non-idempotent-retry")
	assertFindingRulePresent(t, report, "Reliability", "reliability.recoverable-panic")
}

func TestReliabilityCPPDetectsTimeoutCancellationShutdownConcurrencyAndLostContext(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "service.cpp"), `
#include <stdexcept>
#include <thread>

void StartMany(Client& client, Server& server) {
  client.Get("/users");
  std::thread([]{}).detach();
  std::thread([]{}).detach();
  std::thread([]{}).detach();
  std::thread([]{}).detach();
  std::thread([]{}).detach();
  server.Run();
}

void WrapError() {
  try {
    Risky();
  } catch (const std::exception& err) {
    throw std::runtime_error("operation failed");
  }
}
`)

	report, err := codeguard.Run(context.Background(), reliabilityLangConfig("reliability-cpp-parity", dir, "cpp"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Reliability", "reliability.missing-timeout")
	assertFindingRulePresent(t, report, "Reliability", "reliability.missing-cancellation")
	assertFindingRulePresent(t, report, "Reliability", "reliability.missing-concurrency-limit")
	assertFindingRulePresent(t, report, "Reliability", "reliability.missing-graceful-shutdown")
	assertFindingRulePresent(t, report, "Reliability", "reliability.lost-error-context")
}

func TestReliabilityCPPDetectsSwallowedCatch(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "consumer.cpp"), `
#include <stdexcept>

void Consume(Message message) {
  try {
    Process(message);
  } catch (const std::exception& err) {
    return;
  }
}
`)

	report, err := codeguard.Run(context.Background(), reliabilityLangConfig("reliability-cpp-swallowed", dir, "cpp"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Reliability", "reliability.swallowed-error")
}

func TestReliabilityCPPAcceptsBoundedPatterns(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "safe_worker.cpp"), `
#include <chrono>
#include <memory>
#include <thread>

void Run(const std::vector<int>& items, thread_pool& pool) {
  for (auto item : items) {
    pool.enqueue([item] {});
  }
  auto item = std::make_unique<Widget>();
  for (int backoff_attempt = 0; backoff_attempt < 3; ++backoff_attempt) {
    auto retry_attempt_with_backoff = client.Get(idempotency_key);
    std::this_thread::sleep_for(std::chrono::milliseconds(10));
    if (retry_attempt_with_backoff.ok()) return;
  }
}
`)

	report, err := codeguard.Run(context.Background(), reliabilityLangConfig("reliability-cpp-safe", dir, "cpp"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Reliability", "reliability.unbounded-work")
	assertFindingRuleAbsent(t, report, "Reliability", "reliability.resource-leak")
	assertFindingRuleAbsent(t, report, "Reliability", "reliability.retry-without-backoff")
	assertFindingRuleAbsent(t, report, "Reliability", "reliability.unbounded-retry")
	assertFindingRuleAbsent(t, report, "Reliability", "reliability.non-idempotent-retry")
}
