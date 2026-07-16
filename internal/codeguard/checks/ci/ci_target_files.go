package ci

import (
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
)

func isTargetTestFile(language, rel string) bool {
	switch normalizedLanguage(language) {
	case "", "go":
		return strings.HasSuffix(rel, "_test.go")
	case "python":
		return isPythonTestFile(rel)
	case "typescript", "javascript", "ts", "js":
		return isJavaScriptTestFile(rel)
	case "rust", "rs":
		return isRustTestFile(rel)
	case "java":
		return isJavaTestFile(rel)
	case "csharp", "c#", "cs", "dotnet":
		return isCSharpTestFile(rel)
	case "ruby", "rb":
		return isRubyTestFile(rel)
	case "c++", "cpp", "cxx", "cc":
		return isCPPTestFile(rel)
	default:
		return false
	}
}

func isCPPTestFile(rel string) bool {
	if !support.IsCPPPath(rel, true) {
		return false
	}
	slashPath := strings.ToLower(filepath.ToSlash(rel))
	base := filepath.Base(slashPath)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	originalBase := filepath.Base(rel)
	originalStem := strings.TrimSuffix(originalBase, filepath.Ext(originalBase))
	if stem == "test" || stem == "tests" || stem == "unittest" || stem == "unittests" ||
		strings.HasPrefix(stem, "test_") || strings.HasPrefix(stem, "tests_") {
		return true
	}
	for _, suffix := range []string{"_test", "_tests", "_unittest", "_unittests"} {
		if strings.HasSuffix(stem, suffix) {
			return true
		}
	}
	if strings.HasSuffix(originalStem, "Test") || strings.HasSuffix(originalStem, "Tests") {
		return true
	}
	for _, directory := range []string{"test", "tests", "unittest", "unittests"} {
		if strings.HasPrefix(slashPath, directory+"/") || strings.Contains(slashPath, "/"+directory+"/") {
			return true
		}
	}
	return false
}

func normalizedLanguage(language string) string {
	return strings.ToLower(strings.TrimSpace(language))
}

func isPythonTestFile(rel string) bool {
	if !strings.HasSuffix(rel, ".py") {
		return false
	}
	name := filepath.Base(rel)
	return name == "tests.py" || strings.HasPrefix(name, "test_") || strings.HasSuffix(name, "_test.py")
}

func isJavaScriptTestFile(rel string) bool {
	slashPath := filepath.ToSlash(rel)
	if hasJavaScriptTestExtension(slashPath) {
		base := filepath.Base(slashPath)
		if strings.Contains(base, ".test.") || strings.Contains(base, ".spec.") {
			return true
		}
	}
	return (strings.HasPrefix(slashPath, "__tests__/") || strings.Contains(slashPath, "/__tests__/")) && hasJavaScriptTestExtension(slashPath)
}

func hasJavaScriptTestExtension(rel string) bool {
	for _, ext := range []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".mts", ".cts"} {
		if strings.HasSuffix(rel, ext) {
			return true
		}
	}
	return false
}

func isRustTestFile(rel string) bool {
	slashPath := filepath.ToSlash(rel)
	return strings.HasSuffix(slashPath, ".rs") && (strings.HasPrefix(slashPath, "tests/") || strings.Contains(slashPath, "/tests/"))
}

func isJavaTestFile(rel string) bool {
	if !strings.HasSuffix(strings.ToLower(rel), ".java") {
		return false
	}
	base := filepath.Base(rel)
	return strings.HasSuffix(base, "Test.java") || strings.HasSuffix(base, "Tests.java") || strings.HasSuffix(base, "IT.java")
}

func isCSharpTestFile(rel string) bool {
	if !strings.HasSuffix(strings.ToLower(rel), ".cs") {
		return false
	}
	base := filepath.Base(rel)
	slashPath := filepath.ToSlash(rel)
	if strings.HasSuffix(base, "Test.cs") || strings.HasSuffix(base, "Tests.cs") || strings.HasSuffix(base, "Spec.cs") {
		return true
	}
	return strings.HasPrefix(strings.ToLower(slashPath), "tests/") || strings.Contains(strings.ToLower(slashPath), "/tests/")
}

func isRubyTestFile(rel string) bool {
	if !strings.HasSuffix(strings.ToLower(rel), ".rb") {
		return false
	}
	slashPath := filepath.ToSlash(rel)
	base := filepath.Base(slashPath)
	return strings.HasPrefix(slashPath, "test/") ||
		strings.HasPrefix(slashPath, "spec/") ||
		strings.Contains(slashPath, "/test/") ||
		strings.Contains(slashPath, "/spec/") ||
		strings.HasSuffix(base, "_test.rb") ||
		strings.HasSuffix(base, "_spec.rb")
}
