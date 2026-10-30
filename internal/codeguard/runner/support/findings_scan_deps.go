package support

import (
	"path"
	"path/filepath"
	"sort"
	"strings"

	checkSupport "github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// ScanTargetFilesWithDependencies is ScanTargetFiles for an evaluator whose
// findings depend on inputs beyond the scanned file (sibling sources,
// manifests, or resolved target-level state). Those inputs are folded into the
// per-file cache entry, so an edit to any of them invalidates the findings of
// the unchanged files that read it.
func ScanTargetFilesWithDependencies(sc Context, target core.TargetConfig, sectionID string, include func(string) bool, deps checkSupport.FileScanDependencies, evaluator func(string, []byte) []core.Finding) []core.Finding {
	return scanTargetFiles(sc, target, fileScanSpec{
		sectionID:    sectionID,
		include:      include,
		evaluator:    evaluator,
		parallel:     true,
		dependencies: dependencyDigests(sc, target, deps),
	})
}

// dependencyDigests precomputes, once per scan pass, the dependency digest of
// every directory, and returns a read-only lookup safe for the parallel file
// workers. It walks the shared corpus listing once and hashes each declared
// dependency file once, however many scanned files depend on it; without a
// findings cache there is nothing to invalidate, so it computes nothing.
func dependencyDigests(sc Context, target core.TargetConfig, deps checkSupport.FileScanDependencies) func(string) string {
	if sc.Cache == nil || deps.IsZero() {
		return nil
	}
	files, err := sc.corpusFiles(target)
	if err != nil {
		// Without a listing the dependency set is unknown, so no cached entry
		// may be trusted. A non-hash value never equals a stored digest.
		return func(string) string { return "unlisted" }
	}
	sorted := make([]string, len(files))
	copy(sorted, files)
	sort.Strings(sorted)

	var shared strings.Builder
	shared.WriteString("deps-v1|key=" + deps.Key + "\n")
	packages := map[string]*strings.Builder{}
	for _, rel := range sorted {
		inShared := deps.SharedFiles != nil && deps.SharedFiles(rel)
		inPackage := deps.PackageFiles != nil && deps.PackageFiles(rel)
		if !inShared && !inPackage {
			continue
		}
		line := filepath.ToSlash(rel) + "\x00" + dependencyFileHash(sc, target, rel) + "\n"
		if inShared {
			shared.WriteString(line)
		}
		if inPackage {
			dir := dependencyDir(rel)
			builder := packages[dir]
			if builder == nil {
				builder = &strings.Builder{}
				packages[dir] = builder
			}
			builder.WriteString(line)
		}
	}

	base := hashBytes([]byte(shared.String()))
	byDir := make(map[string]string, len(packages))
	for dir, builder := range packages {
		byDir[dir] = hashBytes([]byte(base + "|" + dir + "\n" + builder.String()))
	}
	return func(rel string) string {
		if digest, ok := byDir[dependencyDir(rel)]; ok {
			return digest
		}
		return base
	}
}

func dependencyFileHash(sc Context, target core.TargetConfig, rel string) string {
	data, err := sc.corpusRead(target.Path, rel)
	if err != nil {
		return "unreadable"
	}
	return hashBytes(data)
}

func dependencyDir(rel string) string {
	return path.Dir(filepath.ToSlash(rel))
}
