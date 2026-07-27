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
