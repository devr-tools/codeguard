package support

import (
	"go/ast"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

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

// patternCache memoizes compiled glob patterns keyed by the raw glob string.
// MatchPattern is the hottest compile site in the codebase (per file × per
// pattern during the walk), so compiling once and reusing avoids recompiling
// the same regex repeatedly. A nil value records a glob that failed to compile.
var patternCache sync.Map // map[string]*regexp.Regexp

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

func IsInternalOrCmdFile(path string) bool {
	return isInternalFile(path) || IsCmdFile(path)
}

func isInternalFile(path string) bool {
	return strings.HasPrefix(filepath.ToSlash(path), "internal/")
}

func IsCmdFile(path string) bool {
	return strings.HasPrefix(filepath.ToSlash(path), "cmd/")
}

func IsPublicPackageFile(path string) bool {
	normalized := filepath.ToSlash(path)
	return strings.HasPrefix(normalized, "pkg/")
}

func IsSDKFacadeFile(path string) bool {
	normalized := filepath.ToSlash(path)
	return strings.HasPrefix(normalized, "pkg/codeguard/")
}

func ShouldExclude(rel string, excludes []string) bool {
	defaults := []string{".git/**", ".gocache/**", ".gomodcache/**", ".codeguard/**", "dist/**"}
	for _, pattern := range append(defaults, excludes...) {
		if MatchPattern(pattern, rel) {
			return true
		}
	}
	return false
}

func MatchPattern(pattern string, value string) bool {
	pattern = filepath.ToSlash(strings.TrimSpace(pattern))
	value = filepath.ToSlash(strings.TrimSpace(value))
	if pattern == "" {
		return false
	}
	re, ok := compilePattern(pattern)
	if !ok {
		// An untrusted glob (e.g. from repo config) that translates to an
		// invalid regex matches nothing rather than panicking the scan.
		return false
	}
	return re.MatchString(value)
}

// compilePattern translates a (trimmed, slash-normalized) glob into an anchored
// regex, compiling it at most once per distinct glob. The second return value is
// false when the glob does not yield a valid regex.
func compilePattern(pattern string) (*regexp.Regexp, bool) {
	if cached, ok := patternCache.Load(pattern); ok {
		re, _ := cached.(*regexp.Regexp)
		return re, re != nil
	}
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`.`, `\.`,
		`+`, `\+`,
		`(`, `\(`,
		`)`, `\)`,
		`[`, `\[`,
		`]`, `\]`,
		`{`, `\{`,
		`}`, `\}`,
		`^`, `\^`,
		`$`, `\$`,
	)
	expr := replacer.Replace(pattern)
	expr = strings.ReplaceAll(expr, "**", "§§DOUBLESTAR§§")
	expr = strings.ReplaceAll(expr, "*", `[^/]*`)
	expr = strings.ReplaceAll(expr, "§§DOUBLESTAR§§", `.*`)
	expr = strings.ReplaceAll(expr, "?", `[^/]`)
	re, err := regexp.Compile("^" + expr + "$")
	if err != nil {
		patternCache.Store(pattern, (*regexp.Regexp)(nil))
		return nil, false
	}
	patternCache.Store(pattern, re)
	return re, true
}

func CountLines(data []byte) int {
	return len(strings.Split(strings.TrimRight(string(data), "\n"), "\n"))
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
