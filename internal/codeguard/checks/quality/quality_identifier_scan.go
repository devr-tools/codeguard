package quality

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// sourceIdentifiers memoizes one file's identifier scan across the naming
// detectors. It is per-call state, never shared between goroutines.
type sourceIdentifiers struct {
	source    string
	idents    []identifierAtLine
	words     [][]string
	scanned   bool
	splitDone bool
}

func (s *sourceIdentifiers) all() []identifierAtLine {
	if !s.scanned {
		s.idents = identifiersInSource(s.source)
		s.scanned = true
	}
	return s.idents
}

// wordsPerIdentifier returns each identifier already split into words, in the
// same order as all(). Splitting once per identifier instead of once per
// candidate term is what made the cross-layer detector affordable. Kept
// separate from all() so a detector that only needs the names pays nothing.
func (s *sourceIdentifiers) wordsPerIdentifier() [][]string {
	if s.splitDone {
		return s.words
	}
	idents := s.all()
	s.words = make([][]string, len(idents))
	for i, ident := range idents {
		s.words[i] = splitIdentifierWords(ident.name)
	}
	s.splitDone = true
	return s.words
}

type identifierAtLine struct {
	name string
	line int
}

// identifiersInSource scans for `[A-Za-z_$][A-Za-z0-9_$]*` by hand instead of
// through identifierTokenPattern. The character class is pure ASCII, and a
// UTF-8 continuation byte is never in it, so a byte scan yields exactly the
// matches RE2 does — at a fraction of the cost on whole-file input.
func identifiersInSource(source string) []identifierAtLine {
	out := make([]identifierAtLine, 0, len(source)/8)
	line := 1
	previousMatchStart := 0
	for i := 0; i < len(source); {
		if !isIdentifierStartByte(source[i]) {
			i++
			continue
		}
		start := i
		i++
		for i < len(source) && isIdentifierPartByte(source[i]) {
			i++
		}
		line += strings.Count(source[previousMatchStart:start], "\n")
		out = append(out, identifierAtLine{name: source[start:i], line: line})
		previousMatchStart = start
	}
	return out
}

func isIdentifierStartByte(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c == '_' || c == '$'
}

func isIdentifierPartByte(c byte) bool {
	return isIdentifierStartByte(c) || c >= '0' && c <= '9'
}

// splitIdentifierWords splits on `[_\-\s]+` (Go regexp `\s` is exactly
// "\t\n\f\r "), then on camel-case boundaries. Both separator sets are ASCII,
// so splitting on bytes matches the regexp exactly; empty parts were discarded
// by the caller before and are simply never produced now.
func splitIdentifierWords(name string) []string {
	trimmed := strings.Trim(name, "_$")
	if trimmed == "" {
		return nil
	}
	var words []string
	start := 0
	// Flush the final word after the loop rather than running the loop to
	// i == len(trimmed) as a sentinel. The sentinel form was correct - the
	// index was guarded by a short-circuiting i < len(trimmed) - but it reads
	// as an off-byte-one against length and static analysers flag it as one,
	// so the bound stays strict and the tail is explicit.
	for i := 0; i < len(trimmed); i++ {
		if !isIdentifierWordSeparator(trimmed[i]) {
			continue
		}
		if i > start {
			words = append(words, splitCamelWords(trimmed[start:i])...)
		}
		start = i + 1
	}
	if start < len(trimmed) {
		words = append(words, splitCamelWords(trimmed[start:])...)
	}
	return words
}

func isIdentifierWordSeparator(c byte) bool {
	switch c {
	case '_', '-', '\t', '\n', '\f', '\r', ' ':
		return true
	default:
		return false
	}
}

func splitCamelWords(part string) []string {
	if part == "" {
		return nil
	}
	if isASCIIString(part) {
		return splitCamelWordsASCII(part)
	}
	runes := []rune(part)
	start := 0
	out := make([]string, 0, 3)
	for idx := 1; idx < len(runes); idx++ {
		prev := runes[idx-1]
		cur := runes[idx]
		nextLower := idx+1 < len(runes) && unicode.IsLower(runes[idx+1])
		if unicode.IsLower(prev) && unicode.IsUpper(cur) || unicode.IsUpper(prev) && unicode.IsUpper(cur) && nextLower {
			out = append(out, string(runes[start:idx]))
			start = idx
		}
	}
	out = append(out, string(runes[start:]))
	return out
}

// splitCamelWordsASCII is the allocation-free form of splitCamelWords for the
// overwhelmingly common ASCII case: no []rune copy, and each word is a slice of
// the input rather than a fresh string. unicode.IsLower/IsUpper agree with the
// a-z / A-Z tests for every byte below 0x80, so the split points are identical.
func splitCamelWordsASCII(part string) []string {
	out := make([]string, 0, 3)
	start := 0
	for idx := 1; idx < len(part); idx++ {
		prev := part[idx-1]
		cur := part[idx]
		nextLower := idx+1 < len(part) && isASCIILower(part[idx+1])
		if isASCIILower(prev) && isASCIIUpper(cur) || isASCIIUpper(prev) && isASCIIUpper(cur) && nextLower {
			out = append(out, part[start:idx])
			start = idx
		}
	}
	return append(out, part[start:])
}

func isASCIIString(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

func isASCIILower(c byte) bool { return c >= 'a' && c <= 'z' }

func isASCIIUpper(c byte) bool { return c >= 'A' && c <= 'Z' }

// wordsContain / wordsContainAny take an already-split identifier so callers in
// a loop can split once instead of once per candidate word. They replace
// identifierContainsWord / containsAnyIdentifierWord, which re-split the
// identifier on every question.
func wordsContain(identifierWords []string, word string) bool {
	for _, candidate := range identifierWords {
		if strings.EqualFold(candidate, word) {
			return true
		}
	}
	return false
}

func wordsContainAny(identifierWords []string, words []string) bool {
	for _, word := range words {
		if wordsContain(identifierWords, word) {
			return true
		}
	}
	return false
}

var roleSuffixes = []string{"manager", "helper", "util", "utils", "service", "processor"}

func hasRoleSuffix(name string) bool {
	for _, suffix := range roleSuffixes {
		if len(name) >= len(suffix) && strings.EqualFold(name[len(name)-len(suffix):], suffix) {
			return true
		}
	}
	return false
}
