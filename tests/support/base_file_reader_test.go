package support_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

// baseReaderLargeFileBytes exceeds the 64 MiB git output cap shared by
// runGitCapture and the cat-file batch reader.
const baseReaderLargeFileBytes = 64<<20 + 1

var baseReaderBinary = []byte{0x89, 'P', 'N', 'G', 0x00, 0x01, 0x02, 0xff, 0x00, '\n', 0x00}

// newBaseReaderRepo builds a repo whose working tree differs from `main` by a
// modification, a deletion, a rename, an addition, an unchanged binary file
// and a nested file, so base reads cover every changed-file status.
func newBaseReaderRepo(t *testing.T, withLarge bool) string {
	t.Helper()
	dir := t.TempDir()
	writeScanFile(t, dir, "modified.go", "package main\n\nfunc base() {}\n")
	writeScanFile(t, dir, "deleted.go", "package main\n\nfunc gone() {}\n")
	writeScanFile(t, dir, "old_name.go", "package main\n\nfunc renamed() {}\n")
	writeScanFile(t, dir, "sub/nested.go", "package sub\n\nfunc nested() {}\n")
	writeScanFile(t, dir, "empty.txt", "")
	if err := os.WriteFile(filepath.Join(dir, "image.bin"), baseReaderBinary, 0o644); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	if withLarge {
		if err := os.WriteFile(filepath.Join(dir, "large.txt"), make([]byte, baseReaderLargeFileBytes), 0o644); err != nil {
			t.Fatalf("write large: %v", err)
		}
	}
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "base")

	writeScanFile(t, dir, "modified.go", "package main\n\nfunc head() {}\n")
	if err := os.Remove(filepath.Join(dir, "deleted.go")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	runGit(t, dir, "mv", "old_name.go", "new_name.go")
	writeScanFile(t, dir, "added.go", "package main\n\nfunc added() {}\n")
	return dir
}

func showSC() runnersupport.Context {
	return runnersupport.Context{Opts: core.ScanOptions{Mode: core.ScanModeDiff, BaseRef: "main"}}
}

// TestBaseFileReaderMatchesGitShow checks the batch reader returns exactly
// what the per-file `git show` path returns, for every changed-file status.
func TestBaseFileReaderMatchesGitShow(t *testing.T) {
	dir := newBaseReaderRepo(t, false)
	reader := runnersupport.NewBaseFileReader(context.Background(), "main")
	defer reader.Close()
	target := core.TargetConfig{Name: "repo", Path: dir}

	want := map[string]string{
		"modified.go":   "package main\n\nfunc base() {}\n",
		"deleted.go":    "package main\n\nfunc gone() {}\n",
		"old_name.go":   "package main\n\nfunc renamed() {}\n",
		"sub/nested.go": "package sub\n\nfunc nested() {}\n",
		"image.bin":     string(baseReaderBinary),
		"empty.txt":     "",
	}
	for rel, content := range want {
		got, err := reader.Read(target, rel)
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		if !bytes.Equal(got, []byte(content)) {
			t.Fatalf("read %s = %q, want %q", rel, got, content)
		}
		show, err := runnersupport.ReadBaseFile(context.Background(), showSC(), target, rel)
		if err != nil {
			t.Fatalf("git show %s: %v", rel, err)
		}
		if !bytes.Equal(got, show) {
			t.Fatalf("read %s diverged from git show: %q vs %q", rel, got, show)
		}
	}

	// Paths absent at the base (added, rename destination, nonexistent, a
	// directory) fail just like `git show` does.
	for _, rel := range []string{"added.go", "new_name.go", "nope/missing.go"} {
		if _, err := reader.Read(target, rel); err == nil {
			t.Fatalf("read %s: expected error for path missing at base", rel)
		}
		if _, err := runnersupport.ReadBaseFile(context.Background(), showSC(), target, rel); err == nil {
			t.Fatalf("git show %s: expected error", rel)
		}
	}

	// A missing response must not desynchronise the stream.
	got, err := reader.Read(target, "modified.go")
	if err != nil || string(got) != want["modified.go"] {
		t.Fatalf("read after missing: %q, %v", got, err)
	}
}

func TestBaseFileReaderSubdirectoryTarget(t *testing.T) {
	dir := newBaseReaderRepo(t, false)
	reader := runnersupport.NewBaseFileReader(context.Background(), "main")
	defer reader.Close()
	target := core.TargetConfig{Name: "sub", Path: filepath.Join(dir, "sub")}

	got, err := reader.Read(target, "nested.go")
	if err != nil {
		t.Fatalf("read nested: %v", err)
	}
	if string(got) != "package sub\n\nfunc nested() {}\n" {
		t.Fatalf("unexpected nested content %q", got)
	}
	// Target-relative resolution: a root-level file is not visible as if it
	// lived inside the subdirectory target.
	if _, err := reader.Read(target, "modified.go"); err == nil {
		t.Fatal("expected root file to be missing relative to subdirectory target")
	}
}

// TestBaseFileReaderLargeFileHonoursOutputCap: a base blob above the git
// output cap is rejected like runGitCapture rejects it, and the batch stream
// stays usable afterwards.
func TestBaseFileReaderLargeFileHonoursOutputCap(t *testing.T) {
	if testing.Short() {
		t.Skip("writes a 64 MiB fixture")
	}
	dir := newBaseReaderRepo(t, true)
	reader := runnersupport.NewBaseFileReader(context.Background(), "main")
	defer reader.Close()
	target := core.TargetConfig{Name: "repo", Path: dir}

	if _, err := reader.Read(target, "large.txt"); err == nil || !strings.Contains(err.Error(), "exceeded") {
		t.Fatalf("expected output-cap error for large file, got %v", err)
	}
	if _, err := runnersupport.ReadBaseFile(context.Background(), showSC(), target, "large.txt"); err == nil || !strings.Contains(err.Error(), "exceeded") {
		t.Fatalf("expected git show path to reject large file too, got %v", err)
	}
	got, err := reader.Read(target, "modified.go")
	if err != nil || string(got) != "package main\n\nfunc base() {}\n" {
		t.Fatalf("read after oversized blob: %q, %v", got, err)
	}
}

// TestBaseFileReaderLineBreakPathFallsBack: a path with a newline cannot be
// sent on cat-file's line protocol, so it goes through `git show`.
func TestBaseFileReaderLineBreakPathFallsBack(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("newline file names are not portable")
	}
	dir := t.TempDir()
	writeScanFile(t, dir, "line\nbreak.go", "package main\n")
	writeScanFile(t, dir, "plain.go", "package plain\n")
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "base")

	reader := runnersupport.NewBaseFileReader(context.Background(), "main")
	defer reader.Close()
	target := core.TargetConfig{Name: "repo", Path: dir}
	got, err := reader.Read(target, "line\nbreak.go")
	if err != nil || string(got) != "package main\n" {
		t.Fatalf("read newline path: %q, %v", got, err)
	}
	got, err = reader.Read(target, "plain.go")
	if err != nil || string(got) != "package plain\n" {
		t.Fatalf("read after newline path: %q, %v", got, err)
	}
}

func TestBaseFileReaderReadsAfterCloseAndRejectsBadRef(t *testing.T) {
	dir := newBaseReaderRepo(t, false)
	target := core.TargetConfig{Name: "repo", Path: dir}

	reader := runnersupport.NewBaseFileReader(context.Background(), "main")
	if _, err := reader.Read(target, "modified.go"); err != nil {
		t.Fatalf("read: %v", err)
	}
	reader.Close()
	got, err := reader.Read(target, "deleted.go")
	if err != nil || string(got) != "package main\n\nfunc gone() {}\n" {
		t.Fatalf("read after close: %q, %v", got, err)
	}
	reader.Close() // idempotent

	bad := runnersupport.NewBaseFileReader(context.Background(), "--output=/tmp/x")
	defer bad.Close()
	if _, err = bad.Read(target, "modified.go"); err == nil || !strings.Contains(err.Error(), "invalid base ref") {
		t.Fatalf("expected invalid base ref error, got %v", err)
	}
}

func TestBaseFileReaderCancelledContext(t *testing.T) {
	dir := newBaseReaderRepo(t, false)
	ctx, cancel := context.WithCancel(context.Background())
	reader := runnersupport.NewBaseFileReader(ctx, "main")
	defer reader.Close()
	target := core.TargetConfig{Name: "repo", Path: dir}
	if _, err := reader.Read(target, "modified.go"); err != nil {
		t.Fatalf("read before cancel: %v", err)
	}
	cancel()
	_, err := reader.Read(target, "deleted.go")
	if err == nil {
		t.Fatal("expected read after scan cancellation to fail")
	}
}

// TestBaseFileReaderConcurrentReadsFetchOnce: concurrent sections asking for
// the same path trigger a single fetch, and later reads hit the memo.
func TestBaseFileReaderConcurrentReadsFetchOnce(t *testing.T) {
	var calls atomic.Int32
	release := make(chan struct{})
	missing := errors.New("missing at base")
	fetch := func(_ context.Context, dir string, rel string) ([]byte, error) {
		calls.Add(1)
		<-release
		if rel == "added.go" {
			return nil, missing
		}
		return []byte(dir + ":" + rel), nil
	}
	reader := runnersupport.NewBaseFileReaderWithFetcher(context.Background(), "main", fetch)
	target := core.TargetConfig{Name: "repo", Path: "/repo"}

	const readers = 32
	var started, done sync.WaitGroup
	results := make([][]byte, readers)
	errs := make([]error, readers)
	for i := 0; i < readers; i++ {
		started.Add(1)
		done.Add(1)
		go func() {
			defer done.Done()
			started.Done()
			results[i], errs[i] = reader.Read(target, "a.go")
		}()
	}
	started.Wait()
	close(release)
	done.Wait()

	if got := calls.Load(); got != 1 {
		t.Fatalf("expected one fetch for %d concurrent reads, got %d", readers, got)
	}
	for i := range results {
		if errs[i] != nil || string(results[i]) != "/repo:a.go" {
			t.Fatalf("reader %d got %q, %v", i, results[i], errs[i])
		}
	}

	// Callers own their copy: mutating one result must not leak into others.
	results[0][0] = 'X'
	again, _ := reader.Read(target, "a.go")
	if string(again) != "/repo:a.go" {
		t.Fatalf("memoized content was mutated through a caller's slice: %q", again)
	}

	// Errors are memoized too, and distinct paths/targets are fetched apart.
	for i := 0; i < 3; i++ {
		if _, err := reader.Read(target, "added.go"); !errors.Is(err, missing) {
			t.Fatalf("expected memoized missing error, got %v", err)
		}
	}
	if _, err := reader.Read(core.TargetConfig{Name: "other", Path: "/other"}, "a.go"); err != nil {
		t.Fatalf("read other target: %v", err)
	}
	if got := calls.Load(); got != 3 {
		t.Fatalf("expected 3 fetches (a.go, added.go, other target), got %d", got)
	}
}

// TestBaseFileReaderConcurrentGitReads drives the real batch process from many
// goroutines so -race covers the request lock and the memo together.
func TestBaseFileReaderConcurrentGitReads(t *testing.T) {
	dir := newBaseReaderRepo(t, false)
	reader := runnersupport.NewBaseFileReader(context.Background(), "main")
	defer reader.Close()
	target := core.TargetConfig{Name: "repo", Path: dir}
	paths := []string{"modified.go", "deleted.go", "old_name.go", "sub/nested.go", "image.bin", "added.go"}

	var wg sync.WaitGroup
	failures := make(chan string, 64)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, rel := range paths {
				got, err := reader.Read(target, rel)
				show, showErr := runnersupport.ReadBaseFile(context.Background(), showSC(), target, rel)
				if (err == nil) != (showErr == nil) || !bytes.Equal(got, show) {
					failures <- rel
				}
			}
		}()
	}
	wg.Wait()
	close(failures)
	for rel := range failures {
		t.Fatalf("concurrent read of %s diverged from git show", rel)
	}
}
