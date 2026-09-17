package support

import (
	"bytes"
	"go/ast"
	"io/fs"
	"path/filepath"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// maxScanFileBytes caps the size of an individual scanned repository file that
// codeguard will list and read into memory. Files above this are skipped: the
// repository under scan may be an untrusted pull request, and without a cap a
// single multi-gigabyte file (or many large ones) read fully into the in-memory
// file corpus could exhaust the CI runner (denial of service). Real source files
// are far smaller than this; oversized inputs are almost always generated blobs
// or vendored bundles that are not useful to scan.
const maxScanFileBytes = 32 << 20 // 32 MiB
const maxScanFileCount = 100_000

type FileWalkOptions struct {
	LogicalPath        string
	ScanVendoredSource bool
}

func SummarizeSections(sections []core.SectionResult) core.ReportSummary {
	var summary core.ReportSummary
	for _, section := range sections {
		switch section.Status {
		case core.StatusPass:
			summary.PassedSections++
		case core.StatusWarn:
			summary.WarnedSections++
		case core.StatusFail:
			summary.FailedSections++
		}
		summary.TotalFindings += len(section.Findings)
		summary.SuppressedFindings += section.SuppressedCount
	}
	return summary
}

func WalkFiles(root string, excludes []string, include func(string) bool) ([]string, error) {
	return WalkFilesWithOptions(root, excludes, FileWalkOptions{}, include)
}

func WalkFilesWithOptions(root string, excludes []string, opts FileWalkOptions, include func(string) bool) ([]string, error) {
	files, _, err := walkFilesBounded(root, excludes, opts, include, maxScanFileCount)
	return files, err
}

func walkFilesBounded(root string, excludes []string, opts FileWalkOptions, include func(string) bool, maxFiles int) ([]string, bool, error) {
	var files []string
	truncated := false
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			// The dependency/build policy is applied to the target's logical
			// (repository-relative) root as well, so a target pointed straight
			// at a dependency or output tree is skipped rather than scanned
			// wholesale. shouldExclude already honours a "!" re-include, which
			// is the escape hatch for a repository that really does keep source
			// under one of those names.
			logicalRoot := filepath.ToSlash(filepath.Clean(opts.LogicalPath))
			if logicalRoot != "." && shouldExclude(logicalRoot, excludes, opts) {
				return fs.SkipAll
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		logicalRel := joinLogicalPath(opts.LogicalPath, rel)
		switch walkEntryDecision(path, rel, logicalRel, d.IsDir(), excludes, opts) {
		case walkSkipDir:
			return filepath.SkipDir
		case walkSkipFile:
			return nil
		case walkInclude:
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		// Skip oversized files to bound scan memory (see maxScanFileBytes).
		if info.Size() > maxScanFileBytes {
			return nil
		}
		if include(rel) {
			if len(files) >= maxFiles {
				truncated = true
				return fs.SkipAll
			}
			files = append(files, rel)
		}
		return nil
	})
	return files, truncated, err
}

// walkDecision is the exclusion verdict for one walked entry.
type walkDecision int

const (
	walkInclude walkDecision = iota
	walkSkipFile
	walkSkipDir
)

// walkEntryDecision decides whether a walked entry is scanned, skipped, or
// pruned along with its whole subtree.
//
// rel is the path relative to the directory being walked; logicalRel is the
// same file's repository-relative path when the target root is a subdirectory.
// The built-in default exclusions are matched against rel ONLY, never against
// logicalRel: matching them against the repository path would let an ancestor
// of the target root poison everything beneath it, so a target configured at
// services/app/dist would find nothing at all. (An excluded ancestor is still
// honoured - walkFilesBounded checks the logical root separately and skips the
// walk outright.) User-configured excludes are matched against both forms,
// since they may be written against either.
func walkEntryDecision(path string, rel string, logicalRel string, isDir bool, excludes []string, opts FileWalkOptions) walkDecision {
	if matchesReinclude(rel, excludes) || matchesReinclude(logicalRel, excludes) {
		return walkInclude
	}
	if isDefaultDependencyOrBuildPath(rel, opts.ScanVendoredSource) ||
		matchesUserExclude(rel, excludes) ||
		(logicalRel != rel && matchesUserExclude(logicalRel, excludes)) {
		if isDir {
			// A directory is normally pruned wholesale, which is what keeps the
			// exclusions cheap. It must not be pruned when a re-include could
			// match inside it: "!packages/ui/dist/**" would never be reached if
			// the walk skipped packages/ui/dist first.
			if reincludeCoversSubtree(rel, logicalRel, excludes) {
				return walkInclude
			}
			return walkSkipDir
		}
		return walkSkipFile
	}
	// Ambiguous build-output names (build, out, bin, obj, target, coverage) are
	// pruned only when a sibling manifest corroborates them; see
	// utils_build_output.go for why a name match alone is not enough.
	if isDir && isCorroboratedBuildOutputDir(path) && !reincludeCoversSubtree(rel, logicalRel, excludes) {
		return walkSkipDir
	}
	return walkInclude
}

func reincludeCoversSubtree(rel string, logicalRel string, excludes []string) bool {
	return reincludeMayCoverSubtree(rel, excludes) || reincludeMayCoverSubtree(logicalRel, excludes)
}

// CountLines reports how many lines data spans without allocating. It
// preserves the exact semantics of the previous
// strings.Split(strings.TrimRight(...))-based implementation: all trailing
// newlines are ignored and empty input still counts as one line, so e.g.
// "a\n" and "a\n\n" are 1 line and "" is 1 line.
func CountLines(data []byte) int {
	end := len(data)
	for end > 0 && data[end-1] == '\n' {
		end--
	}
	return bytes.Count(data[:end], []byte{'\n'}) + 1
}

func TypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return TypeName(t.X)
	default:
		return "receiver"
	}
}
