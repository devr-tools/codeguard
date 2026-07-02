package codeguard_test

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// TestFullScanReportIsDeterministicAcrossRuns locks in the ordering contract of
// the parallel scan pipeline: sections run concurrently and each section fans
// its per-file evaluations out on a worker pool, but findings are collected
// into position-indexed slots, so scanning the same tree twice must produce
// byte-identical sections regardless of goroutine scheduling.
func TestFullScanReportIsDeterministicAcrossRuns(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 24; i++ {
		var src strings.Builder
		fmt.Fprintf(&src, "package main\n\nfunc handler%02d() {\n", i)
		for j := 0; j < 8; j++ {
			fmt.Fprintf(&src, "\tprintln(%d)\n", j)
		}
		src.WriteString("}\n")
		writeRepoFile(t, filepath.Join(dir, fmt.Sprintf("file%02d.go", i)), src.String())
	}

	cacheDisabled := false
	cfg := codeguard.ExampleConfig()
	cfg.Name = "determinism"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Cache.Enabled = &cacheDisabled
	// Force one warning per file so ordering across many parallel file scans
	// is actually observable, and keep the scan hermetic (no external tools).
	cfg.Checks.QualityRules.MaxFunctionLines = 5
	cfg.Checks.SecurityRules.GovulncheckMode = "off"

	first, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	second, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}

	if first.Summary.TotalFindings < 24 {
		t.Fatalf("expected at least one finding per file, got %d", first.Summary.TotalFindings)
	}
	if !reflect.DeepEqual(first.Sections, second.Sections) {
		t.Fatalf("sections differ between identical runs:\nfirst:  %+v\nsecond: %+v", first.Sections, second.Sections)
	}
	if !reflect.DeepEqual(first.Summary, second.Summary) {
		t.Fatalf("summaries differ between identical runs:\nfirst:  %+v\nsecond: %+v", first.Summary, second.Summary)
	}
}
