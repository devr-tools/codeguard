package support

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"sync"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// maxBaseMemoBytes bounds how much base-ref content one scan keeps memoized.
// Each individual read is already capped at maxGitOutputBytes; this stops a
// diff touching many large files from pinning all of them for the scan's
// lifetime. Past the budget, results are still shared with concurrent callers
// of the same in-flight read but are not retained afterwards.
const maxBaseMemoBytes = 256 << 20 // 256 MiB

// BaseFileFetcher fetches the base-ref contents of rel, relative to the
// target directory dir. It is the unit of work BaseFileReader memoizes.
type BaseFileFetcher func(ctx context.Context, dir string, rel string) ([]byte, error)

// BaseFileReader serves base-ref file contents for one scan.
//
// Several sections (contracts, performance, quality, change) read the base
// version of the same changed files; previously each read spawned its own
// `git show BASE:path`, so every file was fetched once per section. The
// reader memoizes per (target, path), collapses concurrent reads of the same
// path into one fetch (singleflight), and by default streams every fetch
// through one long-lived `git cat-file --batch` process per target instead of
// one subprocess per file. Callers must Close it when the scan finishes.
type BaseFileReader struct {
	ctx     context.Context
	baseRef string
	fetch   BaseFileFetcher

	mu          sync.Mutex
	entries     map[string]*baseFileEntry
	cachedBytes int

	batches *catFileBatches
}

type baseFileEntry struct {
	done chan struct{}
	data []byte
	err  error
}

// NewBaseFileReader returns a reader that fetches through `git cat-file
// --batch`, falling back to `git show` per file when the batch process cannot
// be used. ctx bounds the lifetime of the batch processes: cancelling it
// kills them.
func NewBaseFileReader(ctx context.Context, baseRef string) *BaseFileReader {
	batches := newCatFileBatches(baseRef)
	reader := newBaseFileReader(ctx, baseRef, batches.fetch)
	reader.batches = batches
	return reader
}

// NewBaseFileReaderWithFetcher returns a memoizing reader around a custom
// fetcher. It exists so the memo and singleflight behaviour can be exercised
// without git.
func NewBaseFileReaderWithFetcher(ctx context.Context, baseRef string, fetch BaseFileFetcher) *BaseFileReader {
	return newBaseFileReader(ctx, baseRef, fetch)
}

func newBaseFileReader(ctx context.Context, baseRef string, fetch BaseFileFetcher) *BaseFileReader {
	return &BaseFileReader{
		ctx:     ctx,
		baseRef: baseRef,
		fetch:   fetch,
		entries: map[string]*baseFileEntry{},
	}
}

// Read returns the contents of a target-relative file at the base ref. The
// returned slice is the caller's own copy.
func (r *BaseFileReader) Read(target core.TargetConfig, rel string) ([]byte, error) {
	if err := ValidateBaseRef(r.baseRef); err != nil {
		return nil, err
	}
	rel = filepath.ToSlash(rel)
	key := target.Path + "\x00" + rel

	r.mu.Lock()
	if entry, ok := r.entries[key]; ok {
		r.mu.Unlock()
		<-entry.done
		return bytes.Clone(entry.data), entry.err
	}
	entry := &baseFileEntry{done: make(chan struct{})}
	r.entries[key] = entry
	r.mu.Unlock()

	entry.data, entry.err = r.fetch(r.ctx, target.Path, rel)

	r.mu.Lock()
	if r.cachedBytes+len(entry.data) > maxBaseMemoBytes {
		delete(r.entries, key)
	} else {
		r.cachedBytes += len(entry.data)
	}
	r.mu.Unlock()
	close(entry.done)

	return bytes.Clone(entry.data), entry.err
}

// Close stops any batch processes. Reads after Close still work; they fall
// back to one `git show` per file.
func (r *BaseFileReader) Close() {
	if r.batches != nil {
		r.batches.close()
	}
}

// showBaseFile is the original one-subprocess-per-file read path.
func showBaseFile(ctx context.Context, dir string, baseRef string, rel string) ([]byte, error) {
	return runGitCapture(ctx, "-C", dir, "show", "--end-of-options", baseRef+":./"+rel)
}

// catFileSpecSafe reports whether rel can be sent on cat-file's line-oriented
// stdin protocol. A path containing a line break would split into two
// requests (and desynchronise the stream), so such paths use `git show`.
func catFileSpecSafe(rel string) bool {
	return !strings.ContainsAny(rel, "\n\r\x00")
}
