package security_test

import (
	"os"
	"path/filepath"
	"testing"

	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

// An untrusted repository must not be able to exhaust scan memory with an
// oversized file: files above the scan size cap are skipped by the walk so no
// section ever reads them into memory. Normal-sized files are still listed.
func TestWalkFilesSkipsOversizedFiles(t *testing.T) {
	root := t.TempDir()

	small := filepath.Join(root, "small.go")
	if err := os.WriteFile(small, []byte("package main\n"), 0o600); err != nil {
		t.Fatalf("write small file: %v", err)
	}

	// A sparse file just over the 32 MiB cap; truncate does not allocate blocks.
	big := filepath.Join(root, "big.go")
	f, err := os.Create(big)
	if err != nil {
		t.Fatalf("create big file: %v", err)
	}
	if err = f.Truncate((32 << 20) + 1); err != nil {
		t.Fatalf("truncate big file: %v", err)
	}
	if err = f.Close(); err != nil {
		t.Fatalf("close big file: %v", err)
	}

	files, err := runnersupport.WalkFiles(root, nil, func(string) bool { return true })
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	got := map[string]bool{}
	for _, rel := range files {
		got[rel] = true
	}
	if !got["small.go"] {
		t.Errorf("expected small.go to be listed, got %v", files)
	}
	if got["big.go"] {
		t.Errorf("oversized big.go must be skipped, got %v", files)
	}
}
