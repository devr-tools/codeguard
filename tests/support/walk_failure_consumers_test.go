package support_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	checkSupport "github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

func lockedWalkTarget(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are not enforced on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root ignores permission bits")
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	locked := filepath.Join(root, "locked")
	if err := os.Mkdir(locked, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o750) })
	return root
}

func hasWalkFailure(section core.SectionResult) bool {
	for _, diagnostic := range section.Diagnostics {
		if diagnostic.ID == "scan.target-walk" {
			return true
		}
	}
	return false
}

// A failed target walk is reported by every section that consumed the
// partial listing and by no other, regardless of finalize order.
func TestWalkFailureAttachesOnlyToListingConsumers(t *testing.T) {
	root := lockedWalkTarget(t)
	target := core.TargetConfig{Name: "repo", Path: root}
	for run := 0; run < 5; run++ {
		sc, err := runnersupport.NewContext(context.Background(), core.Config{Targets: []core.TargetConfig{target}}, core.ScanOptions{Mode: core.ScanModeFull})
		if err != nil {
			t.Fatalf("new context: %v", err)
		}
		consumers := []string{"quality", "security", "design"}
		bystanders := []string{"ci", "supply_chain", "context"}

		checkSupport.ParallelFor(len(consumers), func(i int) {
			if _, err := runnersupport.ListTargetFiles(sc.ForSection(consumers[i]), target); err == nil {
				t.Errorf("%s: listing error = nil, want the walk failure", consumers[i])
			}
		})

		// Finalize concurrently, bystanders first, so attachment cannot
		// depend on finalize order.
		order := append(append([]string{}, bystanders...), consumers...)
		finalized := make([]core.SectionResult, len(order))
		checkSupport.ParallelFor(len(order), func(i int) {
			finalized[i] = runnersupport.FinalizeSection(sc.ForSection(order[i]), order[i], order[i], nil)
		})
		results := make(map[string]core.SectionResult, len(order))
		for i, id := range order {
			results[id] = finalized[i]
		}

		for _, id := range consumers {
			if !hasWalkFailure(results[id]) || results[id].Status != core.StatusFail {
				t.Fatalf("run %d: consumer %s = %+v, want scan.target-walk and a fail", run, id, results[id])
			}
		}
		for _, id := range bystanders {
			if hasWalkFailure(results[id]) {
				t.Fatalf("run %d: section %s never listed the target but reported its walk failure: %+v", run, id, results[id].Diagnostics)
			}
		}
	}
}
