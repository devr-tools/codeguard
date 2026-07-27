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
