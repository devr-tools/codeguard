package quality

import (
	"hash/fnv"
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
		value := strings.ToLower(source[match[0]:match[1]])
		tokens = append(tokens, cloneToken{Value: value, Line: line})
		prev = match[1]
	}
	return tokens
}

func cloneWindowHash(tokens []cloneToken) uint64 {
	hasher := fnv.New64a()
	for _, token := range tokens {
		_, _ = hasher.Write([]byte(token.Value))
		_, _ = hasher.Write([]byte{0})
	}
	return hasher.Sum64()
}

func sharedCloneLength(left []cloneToken, leftStart int, right []cloneToken, rightStart int) int {
	length := 0
	for leftStart+length < len(left) && rightStart+length < len(right) {
		if left[leftStart+length].Value != right[rightStart+length].Value {
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
