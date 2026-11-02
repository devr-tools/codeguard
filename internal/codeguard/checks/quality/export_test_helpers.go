package quality

import (
	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

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
	// Mirror goRepoStyleProfile: count each source on its own, then merge.
	totals := newGoRepoStyleTotals()
	for _, source := range sources {
		fileTotals := newGoRepoStyleTotals()
		fileTotals.add(source)
		totals.merge(fileTotals)
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
	decls := extract(source)
	if len(decls) == 0 {
		return nil
	}
	out := make([]DeclaredNameForTest, 0, len(decls))
	for _, decl := range decls {
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

// CloneTokenSpansForTest returns the [start, end) byte spans the clone lexer
// produces for source, stopping after limit spans when limit >= 0. It mirrors
// regexp.FindAllStringIndex so tests/ can check the lexer against the regex it
// replaced.
func CloneTokenSpansForTest(source string, limit int) [][]int {
	var spans [][]int
	pos := 0
	for limit < 0 || len(spans) < limit {
		start, end, _ := nextCloneToken(source, pos)
		if start < 0 {
			break
		}
		spans = append(spans, []int{start, end})
		pos = end
	}
	return spans
}

// TokenizeCloneTextForTest exposes the clone tokenizer's token text, 1-based
// line numbers and truncation flag to the external test packages.
func TokenizeCloneTextForTest(source string, maxTokens int) (values []string, lines []int, truncated bool) {
	tokens, truncated := tokenizeNormalizedCloneTextBounded(source, maxTokens)
	for _, token := range tokens {
		values = append(values, token.Value)
		lines = append(lines, token.Line)
	}
	return values, lines, truncated
}

// CloneTokenCountForTest runs the clone tokenizer and returns only the token
// count, so benchmarks under tests/ measure tokenizing without copying tokens.
func CloneTokenCountForTest(source string, maxTokens int) int {
	tokens, _ := tokenizeNormalizedCloneTextBounded(source, maxTokens)
	return len(tokens)
}

// CloneSourceForTest is one in-memory file for CloneDocumentsForTest.
type CloneSourceForTest struct {
	Path   string
	Source string
}

// CloneDocumentsForTest runs the clone document collection (batched, parallel
// tokenizing plus in-order budget accounting) over in-memory files visited in
// the given order, and returns each admitted document's path and token count
// and whether the corpus budget truncated the collection.
func CloneDocumentsForTest(files []CloneSourceForTest, maxSourceBytes int, maxTokens int) (paths []string, tokenCounts []int, truncated bool) {
	env := support.Context{
		VisitTargetFiles: func(_ core.TargetConfig, include func(string) bool, visit func(string, []byte)) {
			for _, file := range files {
				if include(file.Path) {
					visit(file.Path, []byte(file.Source))
				}
			}
		},
	}
	docs, truncated := cloneDocumentsForTargetWithBudget(env, core.TargetConfig{Language: "go"}, cloneAnalysisBudget{
		maxSourceBytes: maxSourceBytes,
		maxTokens:      maxTokens,
		maxWindows:     defaultCloneAnalysisBudget.maxWindows,
	})
	for _, doc := range docs {
		paths = append(paths, doc.Path)
		tokenCounts = append(tokenCounts, len(doc.Tokens))
	}
	return paths, tokenCounts, truncated
}

// CloneWindowForTest is one indexed clone window: its rolling hash and the
// (document, token) position it starts at.
type CloneWindowForTest struct {
	Hash     uint64
	Document int
	Token    int
}

// CloneWindowIndexForTest builds the clone window index over documents given
// as per-token hashes and returns the indexed windows in index order (hash
// ascending, each hash's windows in document/token order) plus the truncation
// flag.
func CloneWindowIndexForTest(tokenHashes [][]uint64, threshold int, maxWindows int) ([]CloneWindowForTest, bool) {
	docs := make([]cloneDocument, len(tokenHashes))
	for i, hashes := range tokenHashes {
		tokens := make([]cloneToken, len(hashes))
		for j, hash := range hashes {
			tokens[j] = cloneToken{Hash: hash}
		}
		docs[i] = cloneDocument{Tokens: tokens}
	}
	index, truncated := cloneWindowIndexBounded(docs, threshold, maxWindows)
	windows := make([]CloneWindowForTest, 0, index.len())
	for _, window := range index.windows {
		windows = append(windows, CloneWindowForTest{Hash: window.Hash, Document: int(window.DocIndex), Token: int(window.TokenIndex)})
	}
	return windows, truncated
}

// CloneWindowMultiplierForTest is the rolling-hash multiplier, so a test can
// compute reference window hashes independently.
const CloneWindowMultiplierForTest = cloneWindowMultiplier
