// Package contracts implements API/contract drift detection. Its
// base-comparison rules (Go exported API, OpenAPI, protobuf) compare the diff
// base ref against the working tree and therefore only act in diff mode; the
// destructive-migration rule also runs in full scans.
package contracts

import (
	"context"
	"errors"
	"io/fs"
	"sort"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// Run detects API contract drift between the diff base and the working tree.
func Run(ctx context.Context, env support.Context) core.SectionResult {
	return support.RunTargetSection(ctx, env, "contracts", "API Contracts", findingsForTarget)
}

func findingsForTarget(_ context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	changed := changedFilesForTarget(env, target)
	findings := make([]core.Finding, 0) //nolint:prealloc // count not known up front; each rule appends a variable number
	findings = append(findings, goBreakingFindings(env, target, changed)...)
	findings = append(findings, cppBreakingFindings(env, target, changed)...)
	findings = append(findings, openAPIBreakingFindings(env, target, changed)...)
	findings = append(findings, protoBreakingFindings(env, target, changed)...)
	findings = append(findings, migrationFindings(env, target, changed)...)
	return findings
}

// changedFilesForTarget returns base-vs-head changed files in diff mode. In
// full-scan mode, or when git information is unavailable, it returns nil so
// the base-comparison rules no-op gracefully.
func changedFilesForTarget(env support.Context, target core.TargetConfig) []core.ChangedFile {
	if env.Mode != core.ScanModeDiff || env.ListChangedFiles == nil {
		return nil
	}
	changed, err := env.ListChangedFiles(target)
	if err != nil {
		return nil
	}
	return changed
}

func enabled(flag *bool) bool {
	return flag != nil && *flag
}

// readBase returns the base-ref contents of a file, or nil when unavailable.
func readBase(env support.Context, target core.TargetConfig, rel string) []byte {
	if env.ReadBaseFile == nil {
		return nil
	}
	data, err := env.ReadBaseFile(target, rel)
	if err != nil {
		return nil
	}
	return data
}

// readHead returns the working-tree contents of a file. A missing file reads
// as (nil, true): the head no longer has it, which is what the comparison
// rules report. A file that exists but cannot be read, including one over the
// scan size cap, reads as (nil, false) and callers skip it, because comparing
// the base against an empty head would report every symbol as removed. rel
// comes from the PR diff, so the read goes through the corpus cap rather than
// an unbounded os.ReadFile.
func readHead(env support.Context, target core.TargetConfig, rel string) ([]byte, bool) {
	data, err := support.ReadTargetFileCapped(env, target, rel)
	if err != nil {
		return nil, errors.Is(err, fs.ErrNotExist)
	}
	return data, true
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
