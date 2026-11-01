package support

import (
	"context"
	"slices"
	"sync"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// scanGitMemo memoizes per-scan git facts that several call sites recompute:
// the repo-relative prefix of each target (one `git rev-parse` each, needed by
// both the diff-scope load and the changed-file listing) and each target's
// changed-file list (one `git diff --name-status`, needed by every
// base-comparison section). Both are fixed for the duration of a scan. A nil
// memo computes directly, so callers without a scan Context keep working.
type scanGitMemo struct {
	mu       sync.Mutex
	prefixes map[string]*memoCell[string]
	changed  map[string]*memoCell[[]core.ChangedFile]
}

// memoCell is a singleflight slot: the first caller computes, concurrent
// callers wait on once.
type memoCell[T any] struct {
	once  sync.Once
	value T
	err   error
}

func newScanGitMemo() *scanGitMemo {
	return &scanGitMemo{
		prefixes: map[string]*memoCell[string]{},
		changed:  map[string]*memoCell[[]core.ChangedFile]{},
	}
}

func memoGet[T any](memo *scanGitMemo, cells map[string]*memoCell[T], key string, compute func() (T, error)) (T, error) {
	memo.mu.Lock()
	cell, ok := cells[key]
	if !ok {
		cell = &memoCell[T]{}
		cells[key] = cell
	}
	memo.mu.Unlock()
	cell.once.Do(func() { cell.value, cell.err = compute() })
	return cell.value, cell.err
}

// diffPrefix is a memoized DiffPrefixForTarget.
func (memo *scanGitMemo) diffPrefix(ctx context.Context, dir string) string {
	if memo == nil {
		return DiffPrefixForTarget(ctx, dir)
	}
	prefix, _ := memoGet(memo, memo.prefixes, dir, func() (string, error) {
		return DiffPrefixForTarget(ctx, dir), nil
	})
	return prefix
}

// changedFiles memoizes a target's changed-file list. Each caller receives its
// own copy of the slice.
func (memo *scanGitMemo) changedFiles(dir string, compute func() ([]core.ChangedFile, error)) ([]core.ChangedFile, error) {
	if memo == nil {
		return compute()
	}
	files, err := memoGet(memo, memo.changed, dir, compute)
	return slices.Clone(files), err
}
