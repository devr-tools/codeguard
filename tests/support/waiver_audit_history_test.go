package support_test

import (
	"os"
	"path/filepath"
	"testing"

	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

func TestLoadWaiverAuditHistoryRejectsOversizedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.waiver-audit-history.json")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate((32 << 20) + 1); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	if got := runnersupport.LoadWaiverAuditHistory(path); len(got) != 0 {
		t.Fatalf("expected empty history for oversized file, got %#v", got)
	}
}
