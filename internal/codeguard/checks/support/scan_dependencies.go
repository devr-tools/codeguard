package support

import "github.com/devr-tools/codeguard/internal/codeguard/core"

// FileScanDependencies declares the inputs beyond a file's own content that a
// per-file evaluator reads. The per-file findings cache keys an entry on the
// file's content and the section config; an evaluator that also consults
// sibling files, manifests, or other target-level state must declare that
// state here, or an edit to it replays stale findings for unchanged files.
//
// The runner folds the declared inputs into one digest per directory,
// computed once per scan pass rather than once per file.
type FileScanDependencies struct {
	// Key is an opaque fingerprint of target-level state the evaluator closes
	// over and that is already resolved in memory (for example the SDK facade).
	Key string
	// PackageFiles selects files whose content feeds the analysis of every
	// scanned file in the same directory, such as the sibling sources a
	// package-wide symbol index is built from.
	PackageFiles func(rel string) bool
	// SharedFiles selects files anywhere in the target whose content feeds the
	// analysis of every scanned file, such as go.mod and go.work.
	SharedFiles func(rel string) bool
}

// IsZero reports whether no dependency beyond the file itself is declared.
func (deps FileScanDependencies) IsZero() bool {
	return deps.Key == "" && deps.PackageFiles == nil && deps.SharedFiles == nil
}

// ScanTargetFilesWithDependencies is ScanTargetFiles for an evaluator whose
// findings depend on more than the scanned file. Contexts without the
// dependency-aware hook (unit-test contexts) fall back to ScanTargetFiles,
// which is correct because those contexts run without a findings cache.
func ScanTargetFilesWithDependencies(env Context, target core.TargetConfig, sectionID string, include func(string) bool, deps FileScanDependencies, evaluator func(string, []byte) []core.Finding) []core.Finding {
	if env.ScanTargetFilesWithDependencies != nil {
		return env.ScanTargetFilesWithDependencies(target, sectionID, include, deps, evaluator)
	}
	return env.ScanTargetFiles(target, sectionID, include, evaluator)
}
