package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestQualityCheckWarnsForSyncIOInRequestPath(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "handler.go"), `package sample

import (
	"net/http"
	"os"
)

func handle(w http.ResponseWriter, r *http.Request) {
	_, _ = os.ReadFile("payload.json")
	_, _ = w.Write([]byte("ok"))
}
`)

	cfg := codeguard.ExampleConfig()
	cfg.Name = "quality-sync-io-request-path"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Quality = true
	cfg.Checks.Security = false
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Code Quality", "warn")
	assertFindingRulePresent(t, report, "Code Quality", "quality.sync-io-in-request-path")
}

func TestQualityCheckWarnsForGoroutinesInLoop(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "worker.go"), `package sample

func dispatch(items []int) {
	for _, item := range items {
		go func(value int) {
			_ = value
		}(item)
	}
}
`)

	cfg := codeguard.ExampleConfig()
	cfg.Name = "quality-unbounded-goroutines"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Quality = true
	cfg.Checks.Security = false
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Code Quality", "warn")
	assertFindingRulePresent(t, report, "Code Quality", "quality.unbounded-goroutines-in-loop")
}
