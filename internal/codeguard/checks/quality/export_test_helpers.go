package quality

// SplitIdentifierWordsForTest exposes splitIdentifierWords to the external
// test packages under tests/. The repo keeps tests out of this package, so a
// differential test against the regex implementation this replaced needs a
// seam; splitIdentifierWords itself stays unexported.
func SplitIdentifierWordsForTest(name string) []string {
	return splitIdentifierWords(name)
}

// GoRepoStyleProfileForTest runs goRepoStyleProfile's per-file summation over
// in-memory sources and returns the dominant test framework, error style and
// naming convention. It lets tests/ benchmark the profile without a scan
// context.
func GoRepoStyleProfileForTest(sources []string) (testFramework, errorStyle, naming string) {
	totals := newGoRepoStyleTotals()
	for _, source := range sources {
		totals.add(source)
	}
	profile := totals.profile()
	return profile.testFramework, profile.errorStyle, profile.naming
}

// GoErrorStyleCountsForTest exposes goErrorStyleCounts, whose pkg/errors
// count runs behind a literal pre-gate, so tests/ can check it against an
// ungated oracle.
func GoErrorStyleCountsForTest(source string) map[string]int {
	return goErrorStyleCounts(source)
}

// DeclaredNameForTest is one nameAt result, exported for tests/.
type DeclaredNameForTest struct {
	Name string
	Line int
}

// DeclaredNamesForTest exposes the gated Go, Python and script name
// extractors ("go", "python", "script") to tests/.
func DeclaredNamesForTest(language string, source string) []DeclaredNameForTest {
	extract := map[string]nameExtractor{
		"go":     goDeclaredNames,
		"python": pythonDeclaredNames,
		"script": scriptDeclaredNames,
	}[language]
	var out []DeclaredNameForTest
	for _, decl := range extract(source) {
		out = append(out, DeclaredNameForTest{Name: decl.name, Line: decl.line})
	}
	return out
}

// StyleRegexExprsForTest returns the expressions of the gated style regexes,
// keyed by variable name, so the ungated oracle in tests/ compiles exactly the
// production pattern rather than a copy that could drift.
func StyleRegexExprsForTest() map[string]string {
	return map[string]string{
		"goPkgErrorsPattern":       goPkgErrorsPattern.String(),
		"goErrorfWrapPattern":      goErrorfWrapPattern.String(),
		"goErrorfPattern":          goErrorfPattern.String(),
		"goErrorsNewPattern":       goErrorsNewPattern.String(),
		"goFuncNameAtStartPattern": goFuncNameAtStartPattern.String(),
		"pythonDefNamePattern":     pythonDefNamePattern.String(),
		"scriptFuncNamePattern":    scriptFuncNamePattern.String(),
		"scriptBindingNamePattern": scriptBindingNamePattern.String(),
	}
}
