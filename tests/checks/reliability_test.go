package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func reliabilityConfig(name string, dir string) codeguard.Config {
	cfg := codeguard.ExampleConfig()
	cfg.Name = name
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Quality = false
	cfg.Checks.Design = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.SupplyChain = false
	on := true
	off := false
	cfg.Checks.Reliability = &on
	cfg.Checks.Data = &off
	cfg.Checks.Context = &off
	cfg.Cache.Enabled = &off
	return cfg
}

func TestReliabilityGoDetectsMissingTimeoutAndResourceLeak(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "client.go"), `package sample

import "net/http"

func Fetch(url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	_ = resp
	return nil
}
`)

	report, err := codeguard.Run(context.Background(), reliabilityConfig("reliability-http", dir))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Reliability", "reliability.missing-timeout")
	assertFindingRulePresent(t, report, "Reliability", "reliability.resource-leak")
}

func TestReliabilityGoDetectsCancellationAndUnboundedWork(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "worker.go"), `package sample

import "context"

func Run(ctx context.Context, items []int) {
	child := context.Background()
	_ = child
	for _, item := range items {
		go process(item)
	}
}

func process(int) {}
`)

	report, err := codeguard.Run(context.Background(), reliabilityConfig("reliability-work", dir))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Reliability", "reliability.missing-cancellation")
	assertFindingRulePresent(t, report, "Reliability", "reliability.unbounded-work")
}

func TestReliabilityGoDetectsSwallowedErrorAndPanic(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "errors.go"), `package sample

import "io"

func Copy(dst io.Writer, src io.Reader) {
	_, _ = io.Copy(dst, src)
	panic("copy failed")
}
`)

	report, err := codeguard.Run(context.Background(), reliabilityConfig("reliability-errors", dir))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Reliability", "reliability.swallowed-error")
	assertFindingRulePresent(t, report, "Reliability", "reliability.recoverable-panic")
}

func TestReliabilityGoDetectsRetryRisk(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "retry.go"), `package sample

func ChargeWithRetry(payments Payments, card Card) error {
	for {
		if err := payments.Charge(card); err != nil {
			continue
		}
		return nil
	}
}

type Payments interface {
	Charge(Card) error
}

type Card struct{}
`)

	report, err := codeguard.Run(context.Background(), reliabilityConfig("reliability-retry", dir))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Reliability", "reliability.unbounded-retry")
	assertFindingRulePresent(t, report, "Reliability", "reliability.retry-without-backoff")
	assertFindingRulePresent(t, report, "Reliability", "reliability.non-idempotent-retry")
}

func TestReliabilityPartialFailureAllowsErrorOnlyBatchResultsAcrossLanguages(t *testing.T) {
	cases := []struct {
		name     string
		language string
		file     string
		source   string
	}{
		{
			name:     "go",
			language: "go",
			file:     "records.go",
			source: `package sample

import "log"

func RecordCollectionView(records []Record) error {
	for _, record := range records {
		if err := persist(record); err != nil {
			log.Printf("record collection view failed: %v", err)
			continue
		}
	}
	return nil
}

type Record struct{}
func persist(Record) error { return nil }
`,
		},
		{
			name:     "typescript",
			language: "typescript",
			file:     "records.ts",
			source: `export async function recordCollectionView(records: Record[]): Promise<void> {
  for (const record of records) {
    try {
      await persist(record);
    } catch (error) {
      console.error("record collection view failed", error);
      continue;
    }
  }
}

interface Record {}
declare function persist(record: Record): Promise<void>;
`,
		},
		{
			name:     "python",
			language: "python",
			file:     "records.py",
			source: `import logging

def record_collection_view(records):
    for record in records:
        try:
            persist(record)
        except RuntimeError as error:
            logging.warning("record collection view failed: %s", error)
            continue
    return None
`,
		},
		{
			name:     "cpp",
			language: "cpp",
			file:     "records.cpp",
			source: `#include <iostream>
#include <vector>

void RecordCollectionView(const std::vector<int>& records) {
  for (const auto& record : records) {
    if (!persist(record)) {
      std::cerr << "record collection view failed";
      continue;
    }
  }
}
`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, tc.file), tc.source)

			cfg := reliabilityConfig("reliability-partial-error-only-"+tc.name, dir)
			cfg.Targets[0].Language = tc.language
			report, err := codeguard.Run(context.Background(), cfg)
			if err != nil {
				t.Fatalf("run: %v", err)
			}

			assertFindingRuleAbsent(t, report, "Reliability", "reliability.partial-failure-hidden")
		})
	}
}

func TestReliabilityTypeScriptAllowsCaughtPromiseRejections(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "feature-access-tab.tsx"), `async function fetchNewEditorAccess() {
  const response = await fetch("/api/apps/lmp/new-editor");
  if (!response.ok) {
    throw new Error("failed to load new editor access");
  }
  return response.json();
}

export function FeatureAccessTab() {
  fetchNewEditorAccess().catch((error) => {
    setError(String(error));
  });
  return null;
}

declare function setError(message: string): void;
`)

	cfg := reliabilityConfig("reliability-ts-caught-throw", dir)
	cfg.Targets[0].Language = "typescript"
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Reliability", "reliability.recoverable-panic")
}

func TestReliabilityDedupesOverlappingTargetFindings(t *testing.T) {
	dir := t.TempDir()
	rel := filepath.Join("src", "app", "(app)", "apps", "lmp", "_components", "account-detail", "feature-access-tab.tsx")
	writeFile(t, filepath.Join(dir, rel), `export async function refresh() {
  return fetch("/api/apps/lmp/new-editor");
}
`)

	cfg := reliabilityConfig("reliability-overlap-dedupe", dir)
	cfg.Targets = []codeguard.TargetConfig{
		{Name: "apps", Path: filepath.Join(dir, "src", "app", "(app)", "apps"), Language: "typescript"},
		{Name: "lmp-detail", Path: filepath.Join(dir, "src", "app", "(app)", "apps", "lmp", "_components", "account-detail"), Language: "typescript"},
	}
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if got := countRuleFindings(report, "Reliability", "reliability.missing-timeout"); got != 1 {
		t.Fatalf("expected one deduped missing-timeout finding, got %d", got)
	}
}

func TestReliabilityGoDoesNotFlagBoundedHTTPWithContextAndCleanup(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "client.go"), `package sample

import (
	"context"
	"net/http"
	"time"
)

func Fetch(ctx context.Context, url string) error {
	client := &http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
`)

	report, err := codeguard.Run(context.Background(), reliabilityConfig("reliability-bounded-http", dir))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Reliability", "reliability.missing-timeout")
	assertFindingRuleAbsent(t, report, "Reliability", "reliability.resource-leak")
}

func TestReliabilityGoFlagsZeroClientTimeout(t *testing.T) {
	for _, timeout := range []string{"0", "time.Duration(0)", "0 * time.Second"} {
		t.Run(timeout, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, "client.go"), `package sample

import (
	"net/http"
	"time"
)

func Fetch(req *http.Request) error {
	client := &http.Client{Timeout: `+timeout+`}
	_, err := client.Do(req)
	return err
}
`)

			report, err := codeguard.Run(context.Background(), reliabilityConfig("reliability-zero-timeout", dir))
			if err != nil {
				t.Fatalf("run: %v", err)
			}

			assertFindingRulePresent(t, report, "Reliability", "reliability.missing-timeout")
		})
	}
}
