package quality

import (
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
)

var cloneTokenPattern = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*|\d+|==|!=|<=|>=|&&|\|\||[{}()[\].,:;+*/%<>=!-]`)

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

func tokenizeNormalizedCloneText(source string) []cloneToken {
	matches := cloneTokenPattern.FindAllStringIndex(source, -1)
	if len(matches) == 0 {
		return nil
	}
	tokens := make([]cloneToken, 0, len(matches))
	line := 1
	prev := 0
	for _, match := range matches {
		line += strings.Count(source[prev:match[0]], "\n")
		// Slice the source directly instead of materializing a lowercased
		// copy per token; normalization happens in the hash and in the
		// case-folding comparison, both of which equal the historical
		// lowercase semantics because tokens are ASCII by construction.
		value := source[match[0]:match[1]]
		tokens = append(tokens, cloneToken{Value: value, Hash: cloneTokenHash(value), Line: line})
		prev = match[1]
	}
	return tokens
}

// FNV-1a constants (hash/fnv is not used directly so token hashing can fold
// ASCII case inline without allocating a lowercased copy of each token).
const (
	fnvOffset64 uint64 = 14695981039346694211
	fnvPrime64  uint64 = 1099511628211
)

// cloneTokenHash hashes a token's text with FNV-1a, lowercasing ASCII letters
// on the fly. Tokens only ever contain ASCII (see cloneTokenPattern), so this
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
