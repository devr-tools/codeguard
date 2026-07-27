package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func observabilityConfig(name string, dir string, language string) codeguard.Config {
	cfg := codeguard.ExampleConfig()
	cfg.Name = name
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: language}}
	cfg.Checks.Quality = false
	cfg.Checks.Design = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.SupplyChain = false
	on := true
	off := false
	cfg.Checks.Reliability = &off
	cfg.Checks.Data = &off
	cfg.Checks.Change = &off
	cfg.Checks.Observability = &on
	cfg.Checks.Operations = &off
	cfg.Checks.Context = &off
	cfg.Cache.Enabled = &off
	return cfg
}

func TestObservabilityGoDetectsLoggingAndMetricsRisks(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "handler.go"), `package sample

import "fmt"

func CheckoutHandler(userID string, token string, err error) {
	fmt.Println("checkout failed", err)
	logger.Error(err)
	logger.Info("token", token)
	requests.WithLabelValues(userID).Inc()
}
`)

	report, err := codeguard.Run(context.Background(), observabilityConfig("observability-go", dir, "go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Observability", "observability.unstructured-log")
	assertFindingRulePresent(t, report, "Observability", "observability.error-without-context")
	assertFindingRulePresent(t, report, "Observability", "observability.sensitive-log-data")
	assertFindingRulePresent(t, report, "Observability", "observability.high-cardinality-label")
}

func TestObservabilityDetectsCriticalPathLogIgnoreAndShallowHealth(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "api.py"), `import requests

def payment_handler(err):
    logging.error(err)
    return None

def healthz():
    db = requests.get("http://db")
    return "ok"
`)

	report, err := codeguard.Run(context.Background(), observabilityConfig("observability-python", dir, "python"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Observability", "observability.log-and-ignore")
	assertFindingRulePresent(t, report, "Observability", "observability.shallow-health-check")
	assertFindingRulePresent(t, report, "Observability", "observability.critical-path-uninstrumented")
}

func TestObservabilityTypeScriptAndCPPDetectors(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "consumer.ts"), `export function orderConsumer(event: Event, request_id: string) {
  console.log("received", event)
  logger.error("failed")
  latency.labels({ request_id }).observe(1)
}
`)
	writeFile(t, filepath.Join(dir, "worker.cpp"), `#include <cstdio>

void auth_worker(const char* password) {
  printf("password=%s", password);
}
`)

	tsReport, err := codeguard.Run(context.Background(), observabilityConfig("observability-ts", dir, "typescript"))
	if err != nil {
		t.Fatalf("run ts: %v", err)
	}
	assertFindingRulePresent(t, tsReport, "Observability", "observability.unstructured-log")
	assertFindingRulePresent(t, tsReport, "Observability", "observability.high-cardinality-label")

	cppReport, err := codeguard.Run(context.Background(), observabilityConfig("observability-cpp", dir, "c++"))
	if err != nil {
		t.Fatalf("run cpp: %v", err)
	}
	assertFindingRulePresent(t, cppReport, "Observability", "observability.sensitive-log-data")
}

func TestObservabilityAllowsStructuredInstrumentedCriticalPath(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "handler.go"), `package sample

func CheckoutHandler(ctx context.Context, orderID string, err error) {
	span := tracer.Start(ctx, "checkout")
	defer span.End()
	logger.Error("checkout failed", "operation", "checkout", "order", orderID, "err", err)
	metrics.Counter("checkout_failed").Inc()
}
`)

	report, err := codeguard.Run(context.Background(), observabilityConfig("observability-safe", dir, "go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Observability", "observability.error-without-context")
	assertFindingRuleAbsent(t, report, "Observability", "observability.critical-path-uninstrumented")
}
