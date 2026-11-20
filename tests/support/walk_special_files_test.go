package support_test

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"syscall"
	"testing"
	"time"

	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

// A scanned repository may be an untrusted pull request: the walk must not
// list a symlink that resolves outside the target (codeguard would read and
// report on arbitrary runner files) nor a FIFO (opening it blocks forever).
// Symlinks that stay inside the target are still scanned.
func TestWalkFilesSkipsEscapingSymlinksAndSpecialFiles(t *testing.T) {
	outside := t.TempDir()
	writeScanFile(t, outside, "creds.py", "secret = 1\n")
	root := t.TempDir()
	writeScanFile(t, root, "main.py", "x = 1\n")
	writeScanFile(t, root, filepath.Join("pkg", "real.py"), "y = 1\n")

	symlink(t, filepath.Join(outside, "creds.py"), filepath.Join(root, "leak.py"))
	symlink(t, filepath.Join("..", "..", filepath.Base(outside), "creds.py"), filepath.Join(root, "pkg", "rel_leak.py"))
	symlink(t, filepath.Join(root, "missing.py"), filepath.Join(root, "dangling.py"))
	symlink(t, filepath.Join("pkg", "real.py"), filepath.Join(root, "inside.py"))
	if err := syscall.Mkfifo(filepath.Join(root, "pipe.py"), 0o600); err != nil {
		t.Logf("mkfifo unavailable, skipping FIFO case: %v", err)
	} else {
		fifoTarget := filepath.Join(outside, "pipe")
		if err := syscall.Mkfifo(fifoTarget, 0o600); err == nil {
			symlink(t, fifoTarget, filepath.Join(root, "hang.py"))
		}
	}

	done := make(chan struct{})
	var files []string
	var err error
	go func() {
		defer close(done)
		files, err = runnersupport.WalkFiles(root, nil, func(string) bool { return true })
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("walk did not finish: a special file blocked it")
	}
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	sort.Strings(files)
	want := []string{"inside.py", "main.py", "pkg/real.py"}
	if !reflect.DeepEqual(files, want) {
		t.Fatalf("walked files = %v, want %v", files, want)
	}
}

func symlink(t *testing.T, target string, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
}
