package semantic

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotsForPathsRejectsSymlinkOutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.go")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "leak.go")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if got := snapshotsForPaths(root, []string{"leak.go"}, 24_000); len(got) != 0 {
		t.Fatalf("snapshots = %#v, want symlink escape omitted", got)
	}
}

func TestSnapshotsForPathsAllowsFilesWithinRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "source.go"), []byte("package example"), 0o600); err != nil {
		t.Fatal(err)
	}

	got := snapshotsForPaths(root, []string{"source.go"}, 24_000)
	if len(got) != 1 || got[0].Path != "source.go" || got[0].Content != "package example" {
		t.Fatalf("snapshots = %#v, want source.go snapshot", got)
	}
}
