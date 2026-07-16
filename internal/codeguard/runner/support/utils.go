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
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if ShouldExclude(rel, excludes) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
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
			files = append(files, rel)
		}
		return nil
	})
	return files, err
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
