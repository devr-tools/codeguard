package checks_test

import (
	"fmt"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/quality"
)

// cloneTokenReferencePattern is the regex the clone lexer replaced. It stays
// here as the specification: the lexer must return exactly its spans.
var cloneTokenReferencePattern = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*|\d+|==|!=|<=|>=|&&|\|\||[{}()[\].,:;+*/%<>=!-]`)

// cloneLexerAdversarialInputs targets the places a byte lexer could drift
// from the regex: operator prefixes and overlaps, digit/identifier seams,
// non-ASCII letters and digits the ASCII classes must ignore, invalid and
// truncated UTF-8, and every single byte value.
func cloneLexerAdversarialInputs() []string {
	inputs := []string{
		"", " ", "\n", "\n\n\n", "a", "_", "9", "a9", "9a", "_9_", "0x1F", "1e10", "1.5e-3", "123abc456",
		"===", "!==", "<==", ">==", "=>", "->", "<<=", ">>=", "<<", ">>", "&", "&&", "&&&", "&=", "|", "||", "|||", "|=",
		"!", "!!", "!=", "!==!", "=", "==", "=!=", "<>", "<=>", "a&b|c", "x&&y||z", "a!=b==c<=d>=e",
		"{}()[].,:;+*/%<>=!-", "[\\]", "\\", "\\n", "'a'", "\"str\\\"ing\"", "`tmpl ${x}`", "// comment x\n/* block */",
		"#include <x.h>", "@decorator", "$var", "a?b:c", "a~b^c", "x...y", "--x++", "a ->b",
		"café", "naïve_ident", "αβγ := 1", "変数 = 値", "x\u00a0y", "\u200bzero", "\ufeffbom", "日本9語",
		"\uff10\uff11", "\u0660\u0661", "\U0001F600emoji\U0001F600", "Ⅻ", "ǅ", "ﬁ",
		"\xff", "\xfe\xff", "\xc3", "\xc3(", "\xe2\x82", "\xe2\x82(", "\xf0\x9f\x98", "\xf0\x9f\x98a",
		"\x80abc", "abc\x80", "a\xc0\xafb", "\xed\xa0\x80x", "\xf4\x90\x80\x80y", "a\x00b", "\x00\x01\x7f",
		"\r\n\r\nx\ry", "\t\v\fx",
	}
	var all strings.Builder
	for b := 0; b < 256; b++ {
		inputs = append(inputs, string([]byte{byte(b)}), "a"+string([]byte{byte(b)})+"=")
		all.WriteByte(byte(b))
	}
	inputs = append(inputs, all.String())
	return append(inputs, cloneLexerRandomInputs()...)
}

// cloneLexerRandomInputs splices fragments that sit on token boundaries, with
// a fixed seed so failures reproduce.
func cloneLexerRandomInputs() []string {
	fragments := []string{
		"a", "Z", "_", "0", "9", "=", "!", "<", ">", "&", "|", "-", "+", ".", "[", "]", "\\", "\n", " ",
		"é", "\xff", "\xc3", "\x80", "日", "\U0001F600", "\"", "'", "/", "*", "#", "\x00",
	}
	rng := rand.New(rand.NewSource(20261030))
	inputs := make([]string, 0, 2000)
	for n := 0; n < 2000; n++ {
		var b strings.Builder
		for k := rng.Intn(40); k >= 0; k-- {
			b.WriteString(fragments[rng.Intn(len(fragments))])
		}
		inputs = append(inputs, b.String())
	}
	return inputs
}

// cloneLexerRepoCorpus returns the contents of every .go, .ts and .tsx file in
// the repository, so the comparison covers real code at scale.
func cloneLexerRepoCorpus(t testing.TB) map[string]string {
	t.Helper()
	root := repoRoot(t)
	corpus := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "node_modules", ".gomodcache", "worktrees", "dist":
				return filepath.SkipDir
			}
			return nil
		}
		switch filepath.Ext(path) {
		case ".go", ".ts", ".tsx":
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			corpus[path] = string(data)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk repo corpus: %v", err)
	}
	if len(corpus) < 500 {
		t.Fatalf("repo corpus has %d files, want at least 500", len(corpus))
	}
	return corpus
}

func assertCloneLexerMatchesRegex(t *testing.T, name string, source string) {
	t.Helper()
	for _, limit := range []int{-1, 0, 1, 2, 7} {
		want := cloneTokenReferencePattern.FindAllStringIndex(source, limit)
		got := quality.CloneTokenSpansForTest(source, limit)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s (limit %d): lexer spans differ from regex\n got: %v\nwant: %v", name, limit, got, want)
		}
	}
	assertCloneTokenizeMatchesRegex(t, name, source)
}

// assertCloneTokenizeMatchesRegex checks the full tokenizer output (token
// text, line numbers and the truncation flag) against values derived from the
// regex spans, at several token budgets.
func assertCloneTokenizeMatchesRegex(t *testing.T, name string, source string) {
	t.Helper()
	spans := cloneTokenReferencePattern.FindAllStringIndex(source, -1)
	for _, maxTokens := range []int{-1, 0, 1, len(spans) - 1, len(spans), len(spans) + 1} {
		if maxTokens < -1 {
			continue
		}
		var wantValues []string
		var wantLines []int
		for i, span := range spans {
			if maxTokens >= 0 && i >= maxTokens {
				break
			}
			wantValues = append(wantValues, source[span[0]:span[1]])
			wantLines = append(wantLines, 1+strings.Count(source[:span[0]], "\n"))
		}
		wantTruncated := maxTokens >= 0 && len(spans) > maxTokens
		values, lines, truncated := quality.TokenizeCloneTextForTest(source, maxTokens)
		if !reflect.DeepEqual(values, wantValues) || !reflect.DeepEqual(lines, wantLines) || truncated != wantTruncated {
			t.Fatalf("%s (maxTokens %d): tokenizer differs from regex\n got: %q %v %v\nwant: %q %v %v",
				name, maxTokens, values, lines, truncated, wantValues, wantLines, wantTruncated)
		}
	}
}

func TestCloneLexerMatchesRegexOnAdversarialInputs(t *testing.T) {
	for i, input := range cloneLexerAdversarialInputs() {
		assertCloneLexerMatchesRegex(t, fmt.Sprintf("adversarial #%d %q", i, input), input)
	}
}

func TestCloneLexerMatchesRegexOnRepoCorpus(t *testing.T) {
	corpus := cloneLexerRepoCorpus(t)
	tokens := 0
	for path, source := range corpus {
		want := cloneTokenReferencePattern.FindAllStringIndex(source, -1)
		got := quality.CloneTokenSpansForTest(source, -1)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: lexer produced %d spans, regex %d", path, len(got), len(want))
		}
		tokens += len(want)
		assertCloneTokenizeMatchesRegex(t, path, source)
	}
	t.Logf("compared %d files, %d tokens", len(corpus), tokens)
}

func FuzzCloneLexerMatchesRegex(f *testing.F) {
	for _, input := range cloneLexerAdversarialInputs()[:200] {
		f.Add(input)
	}
	f.Fuzz(func(t *testing.T, source string) {
		assertCloneLexerMatchesRegex(t, "fuzz", source)
	})
}

func cloneLexerBenchmarkSource(b *testing.B) string {
	b.Helper()
	var all strings.Builder
	for _, source := range cloneLexerRepoCorpus(b) {
		all.WriteString(source)
		all.WriteByte('\n')
	}
	return all.String()
}

func BenchmarkCloneTokenizer(b *testing.B) {
	source := cloneLexerBenchmarkSource(b)
	b.SetBytes(int64(len(source)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if quality.CloneTokenCountForTest(source, -1) == 0 {
			b.Fatal("no tokens")
		}
	}
}

// BenchmarkCloneTokenizerRegexReference times only the regex span search the
// lexer replaced, for comparison with BenchmarkCloneTokenizer.
func BenchmarkCloneTokenizerRegexReference(b *testing.B) {
	source := cloneLexerBenchmarkSource(b)
	b.SetBytes(int64(len(source)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if len(cloneTokenReferencePattern.FindAllStringIndex(source, -1)) == 0 {
			b.Fatal("no tokens")
		}
	}
}
