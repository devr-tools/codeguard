package codeguard_test

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// TestFullScanFindingsIndependentOfParallelism scans one frozen mixed
// Go/TypeScript tree serially (GOMAXPROCS=1, so every derived worker pool has
// one worker) and then repeatedly at high parallelism, and requires the exact
// same findings every time. It covers the CPU-derived corpus gates, the
// per-section file workers, the ordered parallel visit behind the
// unresolved-mutation pass, the overlapped TypeScript passes, and the
// parallel AI target pass. Run under -race it is also the data-race check for
// those paths.
func TestFullScanFindingsIndependentOfParallelism(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 24; i++ {
		writeBenchmarkFile(t, filepath.Join(root, "app", fmt.Sprintf("handler_%02d.go", i)), fmt.Sprintf(`package app

import "fmt"

var registry = map[string]int{}

type Handler%d struct{ count int }

func (h *Handler%d) Execute(values []int) (int, error) {
	total := 0
	for _, value := range values {
		if value > 10 {
			if value > 100 {
				if value > 1000 {
					registry["big"]++
				}
			}
		}
		total += value
	}
	h.count++
	if total < 0 {
		return 0, fmt.Errorf("negative total %%d", total)
	}
	return total, nil
}
`, i, i))
		writeBenchmarkFile(t, filepath.Join(root, "web", "src", fmt.Sprintf("view%02d.ts", i)), fmt.Sprintf(`import { helper } from "./missing-helper-%d";

export function compute%d(input: any, items: number[]): number {
  let total = 0;
  try {
    for (const item of items) {
      if (item > 1) { if (item > 2) { if (item > 3) { total += item!; } } }
    }
    state.lastTotal = total;
  } catch (err) {
  }
  return total + helper(input);
}
`, i, i))
	}
	writeBenchmarkFile(t, filepath.Join(root, "go.mod"), "module example.com/parallel\n\ngo 1.23\n")
	writeBenchmarkFile(t, filepath.Join(root, "web", "package.json"), `{"name":"web","private":true}`)
	// Force the per-file TypeScript path so the result does not depend on
	// whether this host has a TypeScript runtime for the semantic engine.
	invalidLib := filepath.Join(root, "invalid-typescript.js")
	writeBenchmarkFile(t, invalidLib, "not typescript")
	t.Setenv("CODEGUARD_TYPESCRIPT_LIB_PATH", invalidLib)

	enabled := true
	cfg := fullScanBenchmarkConfig(root, false, false)
	cfg.Targets = []codeguard.TargetConfig{
		{Name: "go-app", Path: root, Language: "go", Entrypoints: []string{"app"}},
		{Name: "web", Path: filepath.Join(root, "web"), Language: "typescript", Entrypoints: []string{"src"}},
	}
	cfg.Checks.QualityRules.LocalPrecision = &enabled
	cfg.Checks.QualityRules.AIProvenance.Enabled = &enabled

	scan := func(procs int) string {
		t.Helper()
		previous := runtime.GOMAXPROCS(procs)
		defer runtime.GOMAXPROCS(previous)
		report, err := codeguard.RunWithOptions(context.Background(), cfg, codeguard.ScanOptions{Mode: codeguard.ScanModeFull})
		if err != nil {
			t.Fatalf("scan with GOMAXPROCS=%d: %v", procs, err)
		}
		type sectionFindings struct {
			ID       string
			Status   string
			Findings []codeguard.Finding
		}
		sections := make([]sectionFindings, 0, len(report.Sections))
		qualityPaths := map[string]bool{}
		for _, section := range report.Sections {
			sections = append(sections, sectionFindings{ID: section.ID, Status: string(section.Status), Findings: section.Findings})
			if section.ID == "quality" {
				for _, finding := range section.Findings {
					qualityPaths[filepath.Ext(finding.Path)] = true
				}
			}
		}
		// Guard against a vacuous comparison: both languages must reach the
		// quality passes this test exists to exercise.
		if !qualityPaths[".go"] || !qualityPaths[".ts"] {
			t.Fatalf("GOMAXPROCS=%d: expected quality findings on both .go and .ts files, got extensions %v", procs, qualityPaths)
		}
		data, err := json.Marshal(sections)
		if err != nil {
			t.Fatalf("marshal findings: %v", err)
		}
		return string(data)
	}

	want := scan(1)
	if want == "" || want == "[]" {
		t.Fatal("serial scan produced no sections")
	}
	for run := 0; run < 3; run++ {
		if got := scan(64); got != want {
			t.Fatalf("run %d: findings at GOMAXPROCS=64 differ from the serial scan\nserial:   %.600s\nparallel: %.600s", run, want, got)
		}
	}
}
