package quality

import "strings"

// PERF helpers for detectors that would otherwise run the regexp engine over
// every line or function body. Profiling a full scan showed the regexp engine
// taking ~45% of CPU, and the repository already has the precedent: the secret
// scanner's literal pre-gate (builtinGatePasses) took it from 3.2 to 99 MB/s,
// because an alternation whose branches start with \b or (?i) defeats RE2's
// literal-prefix fast path.
//
// Both helpers below are exact, not approximate:
//   - containsAnyFold is an ASCII case-insensitive strings.Contains, which is
//     what (?i) means for the ASCII literals these gates hold.
//   - containsWordAny applies Go regexp's own \b definition: a boundary sits
//     where a word character ([0-9A-Za-z_]) meets a non-word character or an
//     edge of the string.

// containsAnyFold reports whether value may contain one of literals under
// (?i), comparing ASCII letters case-insensitively. literals must already be
// lowercase ASCII.
//
// Go's (?i) uses Unicode simple folding, so `s` also matches U+017F and `k`
// also matches U+212A. Rather than model that, any value holding a non-ASCII
// byte is reported as a possible match, which keeps the gate a strict superset
// of the pattern. Source bodies are ASCII in practice, so the gate still fires.
func containsAnyFold(value string, literals []string) bool {
	lowered, ascii := asciiLower(value)
	if !ascii {
		return true
	}
	for _, literal := range literals {
		if strings.Contains(lowered, literal) {
			return true
		}
	}
	return false
}

// asciiLower lowercases ASCII letters, returning the input unchanged (and
// allocating nothing) when it holds none. The second result is false as soon as
// a byte >= 0x80 is seen, and the first result is then meaningless.
func asciiLower(value string) (string, bool) {
	upper := -1
	for i := 0; i < len(value); i++ {
		c := value[i]
		if c >= 0x80 {
			return "", false
		}
		if upper < 0 && c >= 'A' && c <= 'Z' {
			upper = i
		}
	}
	if upper < 0 {
		return value, true
	}
	buf := []byte(value)
	for i := upper; i < len(buf); i++ {
		if buf[i] >= 'A' && buf[i] <= 'Z' {
			buf[i] += 'a' - 'A'
		}
	}
	return string(buf), true
}

// containsWordAny reports whether value contains any of words delimited by
// regexp word boundaries, i.e. it is equivalent to `\b(w1|w2|...)\b`. Every
// word must be ASCII and already lowercase, and value must already be
// lowercased by the caller (the patterns this replaces matched lowered input).
func containsWordAny(value string, words []string) bool {
	for _, word := range words {
		if containsWordBoundary(value, word) {
			return true
		}
	}
	return false
}

func containsWordBoundary(value string, word string) bool {
	if word == "" {
		return false
	}
	for offset := 0; ; {
		idx := strings.Index(value[offset:], word)
		if idx < 0 {
			return false
		}
		start := offset + idx
		end := start + len(word)
		beforeOK := start == 0 || !isRegexpWordByte(value[start-1])
		afterOK := end == len(value) || !isRegexpWordByte(value[end])
		if beforeOK && afterOK {
			return true
		}
		offset = start + 1
	}
}

func isRegexpWordByte(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c == '_'
}
