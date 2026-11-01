package contextgen

import (
	"path"
	"sort"
	"strings"

	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

// treeStats is one pass over the repository's files, aggregated per
// top-level directory ("." holds files at the root).
type treeStats struct {
	dirs  map[string]*dirStats
	tests map[string]*testStats
}

type dirStats struct {
	files      int
	byLanguage map[string]int
	byExt      map[string]int
	// subdirMarkers maps a marker file name (see subdirMarkerFiles) to the
	// immediate subdirectories holding it.
	subdirMarkers map[string]map[string]struct{}
}

type testStats struct {
	files     int
	languages map[string]struct{}
}

// walkExcludes skips hidden directories (tool state, editor and agent
// worktrees); the scanner's dependency and build exclusions and .gitignore
// apply in addition.
var walkExcludes = []string{".*", "**/.*"}

// subdirMarkerFiles are the files whose presence in an immediate
// subdirectory identifies what a top-level directory holds.
var subdirMarkerFiles = map[string]bool{"main.go": true, "package.json": true, "Chart.yaml": true, "Cargo.toml": true, "pyproject.toml": true}

var sourceLanguageByExt = map[string]string{
	".go": "Go", ".py": "Python",
	".ts": "TypeScript", ".tsx": "TypeScript", ".mts": "TypeScript", ".cts": "TypeScript",
	".js": "JavaScript", ".jsx": "JavaScript", ".mjs": "JavaScript", ".cjs": "JavaScript",
	".rs": "Rust", ".java": "Java", ".rb": "Ruby", ".cs": "C#",
	".cc": "C++", ".cpp": "C++", ".cxx": "C++", ".hpp": "C++", ".hh": "C++", ".hxx": "C++",
}

func walkTree(root string) (treeStats, error) {
	stats := treeStats{dirs: map[string]*dirStats{}, tests: map[string]*testStats{}}
	_, err := runnersupport.WalkFilesWithOptions(root, walkExcludes, runnersupport.FileWalkOptions{RespectGitignore: true}, func(rel string) bool {
		stats.record(rel)
		return false
	})
	return stats, err
}

func (t treeStats) record(rel string) {
	top, rest := ".", rel
	if idx := strings.IndexByte(rel, '/'); idx > 0 {
		top, rest = rel[:idx], rel[idx+1:]
	}
	dir := t.dirs[top]
	if dir == nil {
		dir = &dirStats{byLanguage: map[string]int{}, byExt: map[string]int{}, subdirMarkers: map[string]map[string]struct{}{}}
		t.dirs[top] = dir
	}
	dir.files++
	ext := strings.ToLower(path.Ext(rel))
	dir.byExt[ext]++
	lang, isSource := sourceLanguageByExt[ext]
	if isSource {
		dir.byLanguage[lang]++
	}
	if parts := strings.Split(rest, "/"); len(parts) == 2 && subdirMarkerFiles[parts[1]] {
		markers := dir.subdirMarkers[parts[1]]
		if markers == nil {
			markers = map[string]struct{}{}
			dir.subdirMarkers[parts[1]] = markers
		}
		markers[parts[0]] = struct{}{}
	}
	if testLang, ok := testFileLanguage(rel, top); ok && !isFixturePath(rel) {
		test := t.tests[top]
		if test == nil {
			test = &testStats{languages: map[string]struct{}{}}
			t.tests[top] = test
		}
		test.files++
		test.languages[testLang] = struct{}{}
	}
}

// isFixturePath reports files under a fixture tree: a test-named file there
// is sample input for a test, not a test.
func isFixturePath(rel string) bool {
	for _, segment := range strings.Split(path.Dir(rel), "/") {
		switch segment {
		case "testdata", "fixtures", "__fixtures__":
			return true
		}
	}
	return false
}

// testFileLanguage recognizes test files by each ecosystem's own naming
// convention, never by guessing from content.
func testFileLanguage(rel string, top string) (string, bool) {
	base := path.Base(rel)
	ext := strings.ToLower(path.Ext(base))
	stem := strings.TrimSuffix(base, path.Ext(base))
	lang, isSource := sourceLanguageByExt[ext]
	if !isSource {
		return "", false
	}
	switch lang {
	case "Go":
		return lang, strings.HasSuffix(base, "_test.go")
	case "Python":
		return lang, strings.HasPrefix(base, "test_") || strings.HasSuffix(stem, "_test")
	case "TypeScript", "JavaScript":
		return lang, strings.HasSuffix(stem, ".test") || strings.HasSuffix(stem, ".spec") || strings.Contains(rel, "/__tests__/") || top == "__tests__"
	case "Rust":
		return lang, top == "tests"
	case "Java":
		return lang, strings.HasSuffix(stem, "Test") || strings.HasSuffix(stem, "Tests")
	case "Ruby":
		return lang, strings.HasSuffix(stem, "_spec") || strings.HasSuffix(stem, "_test")
	default:
		return "", false
	}
}

func (t treeStats) testLocations() []TestLocation {
	locations := make([]TestLocation, 0, len(t.tests))
	for top, stats := range t.tests {
		loc := TestLocation{Path: top, Files: stats.files}
		if top != "." {
			loc.Path = top + "/"
		}
		for lang := range stats.languages {
			loc.Languages = append(loc.Languages, lang)
		}
		sort.Strings(loc.Languages)
		locations = append(locations, loc)
	}
	sort.Slice(locations, func(i, j int) bool {
		if locations[i].Files != locations[j].Files {
			return locations[i].Files > locations[j].Files
		}
		return locations[i].Path < locations[j].Path
	})
	return locations
}

func (t treeStats) hasTestFiles(lang string) bool {
	for _, stats := range t.tests {
		if _, ok := stats.languages[lang]; ok {
			return true
		}
	}
	return false
}

// languagesSummary renders a directory's source counts, largest first, such
// as "12 Go files, 2 Python files".
func (d *dirStats) languagesSummary() string {
	type count struct {
		lang  string
		files int
	}
	counts := make([]count, 0, len(d.byLanguage))
	for lang, files := range d.byLanguage {
		counts = append(counts, count{lang, files})
	}
	sort.Slice(counts, func(i, j int) bool {
		if counts[i].files != counts[j].files {
			return counts[i].files > counts[j].files
		}
		return counts[i].lang < counts[j].lang
	})
	parts := make([]string, 0, len(counts))
	for _, c := range counts {
		parts = append(parts, pluralize(c.files, c.lang+" file"))
	}
	if len(parts) == 0 {
		return pluralize(d.files, "file")
	}
	return strings.Join(parts, ", ")
}

// subdirsWith counts immediate subdirectories holding the marker file.
func (d *dirStats) subdirsWith(marker string) int {
	return len(d.subdirMarkers[marker])
}

func (d *dirStats) extShare(exts ...string) float64 {
	if d.files == 0 {
		return 0
	}
	matched := 0
	for _, ext := range exts {
		matched += d.byExt[ext]
	}
	return float64(matched) / float64(d.files)
}
