package quality

import (
	"path/filepath"
	"strings"
)

func rustComplexity(body string) int {
	return keywordComplexity(body, []string{" if ", " match ", " for ", " while ", " loop ", "&&", "||"})
}

func braceComplexity(body string) int {
	return keywordComplexity(body, []string{" if ", " for ", " foreach ", " while ", " catch ", " case ", "&&", "||", "?"})
}

func rubyComplexity(body string) int {
	return keywordComplexity(body, []string{" if ", " unless ", " elsif ", " when ", " rescue ", "&&", "||"})
}

func keywordComplexity(body string, tokens []string) int {
	complexity := 1
	padded := " " + strings.ReplaceAll(body, "\n", " ") + " "
	for _, token := range tokens {
		complexity += strings.Count(padded, token)
	}
	return complexity
}

func isRustFile(rel string) bool {
	return strings.EqualFold(filepath.Ext(rel), ".rs")
}

func isJavaFile(rel string) bool {
	return strings.EqualFold(filepath.Ext(rel), ".java")
}

func isCSharpFile(rel string) bool {
	return strings.EqualFold(filepath.Ext(rel), ".cs")
}

func isRubyFile(rel string) bool {
	return strings.EqualFold(filepath.Ext(rel), ".rb")
}
