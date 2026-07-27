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
