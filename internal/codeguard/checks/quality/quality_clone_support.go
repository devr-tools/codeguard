package quality

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
)

func cloneExcludedPath(language string, rel string) bool {
	slash := filepath.ToSlash(strings.ToLower(rel))
	if strings.HasPrefix(slash, "tests/") || strings.Contains(slash, "/tests/") {
		return true
	}

	switch support.NormalizedLanguage(language) {
	case "", "go":
		return strings.HasSuffix(slash, "_test.go")
	case "python", "py":
		base := path.Base(slash)
		return base == "tests.py" || strings.HasPrefix(base, "test_") || strings.HasSuffix(base, "_test.py")
	case "typescript", "javascript", "ts", "tsx", "js", "jsx":
		base := path.Base(slash)
		return strings.Contains(base, ".test.") || strings.Contains(base, ".spec.") ||
			strings.HasPrefix(slash, "__tests__/") || strings.Contains(slash, "/__tests__/")
	case "java":
		base := path.Base(slash)
		return strings.HasSuffix(base, "test.java") || strings.HasSuffix(base, "tests.java") || strings.HasSuffix(base, "it.java")
	case "csharp", "c#", "cs", "dotnet":
		base := path.Base(slash)
		return strings.HasSuffix(base, "test.cs") || strings.HasSuffix(base, "tests.cs") || strings.HasSuffix(base, "spec.cs")
	case "ruby", "rb":
		base := path.Base(slash)
		return strings.HasPrefix(slash, "test/") || strings.HasPrefix(slash, "spec/") ||
			strings.Contains(slash, "/test/") || strings.Contains(slash, "/spec/") ||
			strings.HasSuffix(base, "_test.rb") || strings.HasSuffix(base, "_spec.rb")
	default:
		return false
	}
}

func cloneIncludeForLanguage(language string) func(string) bool {
	switch support.NormalizedLanguage(language) {
	case "", "go":
		return func(rel string) bool { return strings.HasSuffix(strings.ToLower(rel), ".go") }
	case "python", "py":
		return func(rel string) bool { return strings.HasSuffix(strings.ToLower(rel), ".py") }
	case "typescript", "javascript", "ts", "tsx", "js", "jsx":
		return isTypeScriptLikeFile
	case "rust", "rs":
		return isRustFile
	case "java":
		return isJavaFile
	case "csharp", "c#", "cs", "dotnet":
		return isCSharpFile
	case "ruby", "rb":
		return isRubyFile
	default:
		return func(string) bool { return false }
	}
}

func tokenizeNormalizedCloneTextBounded(source string, maxTokens int) ([]cloneToken, bool) {
	var tokens []cloneToken
	line := 1
	pos := 0
	for {
		start, end, newlines := nextCloneToken(source, pos)
		if start < 0 {
			return tokens, false
		}
		if maxTokens >= 0 && len(tokens) == maxTokens {
			return tokens, true
		}
		if tokens == nil {
			tokens = make([]cloneToken, 0, cloneTokenCapacity(len(source), maxTokens))
		}
		line += newlines
		// Slice the source directly instead of materializing a lowercased
		// copy per token; normalization happens in the hash and in the
		// case-folding comparison, both of which equal the historical
		// lowercase semantics because tokens are ASCII by construction.
		value := source[start:end]
		tokens = append(tokens, cloneToken{Value: value, Hash: cloneTokenHash(value), Line: line})
		pos = end
	}
}

// cloneTokenCapacity estimates the token count up front so append rarely
// regrows the slice; source code averages several bytes per token.
func cloneTokenCapacity(sourceLen int, maxTokens int) int {
	estimate := sourceLen/5 + 1
	if maxTokens >= 0 && estimate > maxTokens {
		return maxTokens
	}
	return estimate
}

// cloneTokenClass classifies each byte for the clone lexer. The lexer is a
// hand-written replacement for the regex
//
//	[A-Za-z_][A-Za-z0-9_]*|\d+|==|!=|<=|>=|&&|\|\||[{}()[\].,:;+*/%<>=!-]
//
// which tokenized every file on the scan's critical path and dominated clone
// detection. Every byte the regex can match is ASCII, and ASCII bytes never
// occur inside a multi-byte or invalid UTF-8 sequence, so scanning bytes gives
// exactly the regex's spans (tests/checks keeps the regex as the reference).
const (
	cloneByteSkip uint8 = iota
	cloneByteIdent
	cloneByteDigit
	cloneBytePunct
	// cloneBytePair marks bytes that may open a two-byte operator; '=', '!',
	// '<' and '>' are also single-byte tokens, '&' and '|' are not.
	cloneBytePair
)

var cloneByteClasses = buildCloneByteClasses()

func buildCloneByteClasses() [256]uint8 {
	var classes [256]uint8
	for b := 'a'; b <= 'z'; b++ {
		classes[b] = cloneByteIdent
		classes[b-'a'+'A'] = cloneByteIdent
	}
	classes['_'] = cloneByteIdent
	for b := '0'; b <= '9'; b++ {
		classes[b] = cloneByteDigit
	}
	for _, b := range []byte("{}()[].,:;+*/%-") {
		classes[b] = cloneBytePunct
	}
	for _, b := range []byte("=!<>&|") {
		classes[b] = cloneBytePair
	}
	return classes
}

// nextCloneToken returns the span of the first token at or after pos and the
// number of newlines skipped before it, or start -1 when none remains.
func nextCloneToken(source string, pos int) (start int, end int, newlines int) {
	for i := pos; i < len(source); i++ {
		switch cloneByteClasses[source[i]] {
		case cloneByteIdent:
			return i, scanCloneRun(source, i+1, cloneByteIdent, cloneByteDigit), newlines
		case cloneByteDigit:
			return i, scanCloneRun(source, i+1, cloneByteDigit, cloneByteDigit), newlines
		case cloneBytePunct:
			return i, i + 1, newlines
		case cloneBytePair:
			if width := cloneOperatorWidth(source, i); width > 0 {
				return i, i + width, newlines
			}
		default:
			if source[i] == '\n' {
				newlines++
			}
		}
	}
	return -1, -1, newlines
}

// scanCloneRun returns the end of the run of bytes in either class from i.
func scanCloneRun(source string, i int, first uint8, second uint8) int {
	for i < len(source) {
		class := cloneByteClasses[source[i]]
		if class != first && class != second {
			break
		}
		i++
	}
	return i
}

// cloneOperatorWidth returns the operator length at i: 2 for ==, !=, <=, >=,
// && and ||, 1 for a lone = ! < >, and 0 for a lone & or | (not a token).
func cloneOperatorWidth(source string, i int) int {
	b := source[i]
	if i+1 < len(source) {
		next := source[i+1]
		if next == '=' && b != '&' && b != '|' {
			return 2
		}
		if next == b && (b == '&' || b == '|' || b == '=') {
			return 2
		}
	}
	if b == '&' || b == '|' {
		return 0
	}
	return 1
}

// FNV-1a constants (hash/fnv is not used directly so token hashing can fold
// ASCII case inline without allocating a lowercased copy of each token).
const (
	fnvOffset64 uint64 = 14695981039346694211
	fnvPrime64  uint64 = 1099511628211
)

// cloneTokenHash hashes a token's text with FNV-1a, lowercasing ASCII letters
// on the fly. Tokens only ever contain ASCII (see cloneByteClasses), so this
// equals hashing strings.ToLower(text).
func cloneTokenHash(text string) uint64 {
	hash := fnvOffset64
	for i := 0; i < len(text); i++ {
		b := text[i]
		if 'A' <= b && b <= 'Z' {
			b += 'a' - 'A'
		}
		hash ^= uint64(b)
		hash *= fnvPrime64
	}
	return hash
}

// cloneTokenTextEqual reports whether two token texts are equal ignoring ASCII
// case — exactly the historical comparison of lowercased token values, since
// token text is ASCII-only.
func cloneTokenTextEqual(a string, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if 'A' <= ca && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if 'A' <= cb && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

func sharedCloneLength(left []cloneToken, leftStart int, right []cloneToken, rightStart int) int {
	length := 0
	for leftStart+length < len(left) && rightStart+length < len(right) {
		l, r := left[leftStart+length], right[rightStart+length]
		// Equal normalized text implies equal hashes, so a hash mismatch is a
		// definitive inequality; the text comparison then guards against hash
		// collisions, keeping the match semantics identical to comparing
		// lowercased token values.
		if l.Hash != r.Hash || !cloneTokenTextEqual(l.Value, r.Value) {
			break
		}
		length++
	}
	return length
}

func cloneRangesOverlap(startA int, lenA int, startB int, lenB int) bool {
	endA := startA + lenA
	endB := startB + lenB
	return startA < endB && startB < endA
}
