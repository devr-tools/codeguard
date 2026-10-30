package checks_test

import (
	"regexp"
	"strings"
	"testing"
	"testing/quick"
	"unicode"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/quality"
)

// referenceSeparator is the pattern splitIdentifierWords replaced when the
// naming detectors stopped recompiling regexes per call.
var referenceSeparator = regexp.MustCompile(`[_\-\s]+`)

// referenceSplit is the original regex implementation, kept here as the
// oracle. The byte-scan rewrite must agree with it exactly.
func referenceSplit(name string) []string {
	trimmed := strings.Trim(name, "_$")
	if trimmed == "" {
		return nil
	}
	var words []string
	for _, part := range referenceSeparator.Split(trimmed, -1) {
		if part == "" {
			continue
		}
		words = append(words, referenceCamel(part)...)
	}
	return words
}

func referenceCamel(part string) []string {
	if part == "" {
		return nil
	}
	runes := []rune(part)
	start := 0
	out := make([]string, 0, 3)
	for idx := 1; idx < len(runes); idx++ {
		prev, cur := runes[idx-1], runes[idx]
		nextLower := idx+1 < len(runes) && unicode.IsLower(runes[idx+1])
		if unicode.IsLower(prev) && unicode.IsUpper(cur) || unicode.IsUpper(prev) && unicode.IsUpper(cur) && nextLower {
			out = append(out, string(runes[start:idx]))
			start = idx
		}
	}
	return append(out, string(runes[start:]))
}

// TestSplitIdentifierWordsHandlesBoundaryInput pins the index arithmetic at
// every boundary the splitter has: an empty result after trimming, a string
// that is all separators, leading and trailing separators, and invalid UTF-8.
// A bot reported the previous sentinel loop as an off-by-one against length;
// it was not one, but these are the inputs that would have proved it either
// way, so they stay.
func TestSplitIdentifierWordsHandlesBoundaryInput(t *testing.T) {
	for _, name := range []string{
		"", "_", "__", "$", "_$", "$_", "a", "_a", "a_", "_a_",
		"---", "   ", "\t\n\f\r ", "a-b", "a--b", "-a-", "a_b-c d",
		"HTTPServer", "parseURLFrom", "x1", "_1", "日本語", "a日b_c",
		"\xff\xfe", "a\xffb_c",
		strings.Repeat("a_", 4096), strings.Repeat("_", 4096),
		"A" + strings.Repeat("-", 1024) + "B",
	} {
		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Errorf("splitIdentifierWords panicked on %q: %v", name, recovered)
				}
			}()
			if got, want := quality.SplitIdentifierWordsForTest(name), referenceSplit(name); !equalWords(got, want) {
				t.Errorf("split %q = %q, want %q", name, got, want)
			}
		}()
	}
}

// TestSplitIdentifierWordsMatchesRegexOracle is the differential half: the
// byte scan must agree with the regex it replaced on arbitrary input, not
// only on the cases someone thought to write down.
func TestSplitIdentifierWordsMatchesRegexOracle(t *testing.T) {
	agrees := func(name string) bool {
		return equalWords(quality.SplitIdentifierWordsForTest(name), referenceSplit(name))
	}
	if err := quick.Check(agrees, &quick.Config{MaxCount: 50000}); err != nil {
		t.Errorf("byte scan disagrees with the regex oracle: %v", err)
	}
}

func equalWords(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
