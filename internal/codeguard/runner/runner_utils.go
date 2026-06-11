package runner

import (
	"go/ast"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func summarizeSections(sections []core.SectionResult) core.ReportSummary {
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

func walkFiles(root string, excludes []string, include func(string) bool) ([]string, error) {
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
		if shouldExclude(rel, excludes) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if include(rel) {
			files = append(files, rel)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

func isInternalOrCmdFile(path string) bool {
	return isInternalFile(path) || isCmdFile(path)
}

func isInternalFile(path string) bool {
	return strings.HasPrefix(filepath.ToSlash(path), "internal/")
}

func isCmdFile(path string) bool {
	return strings.HasPrefix(filepath.ToSlash(path), "cmd/")
}

func isServicePackageFile(path string) bool {
	return strings.HasPrefix(filepath.ToSlash(path), "codeguard/")
}

func shouldExclude(rel string, excludes []string) bool {
	defaults := []string{".git/**", ".gocache/**", ".gomodcache/**", ".codeguard/**", "dist/**"}
	for _, pattern := range append(defaults, excludes...) {
		if matchPattern(pattern, rel) {
			return true
		}
	}
	return false
}

func matchPattern(pattern string, value string) bool {
	pattern = filepath.ToSlash(strings.TrimSpace(pattern))
	value = filepath.ToSlash(strings.TrimSpace(value))
	if pattern == "" {
		return false
	}
	replacer := strings.NewReplacer(
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
	re := regexp.MustCompile("^" + expr + "$")
	return re.MatchString(value)
}

func countLines(data []byte) int {
	return len(strings.Split(strings.TrimRight(string(data), "\n"), "\n"))
}

func typeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return typeName(t.X)
	default:
		return "receiver"
	}
}

func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}
