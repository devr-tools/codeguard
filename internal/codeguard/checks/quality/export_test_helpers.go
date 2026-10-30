package quality

// SplitIdentifierWordsForTest exposes splitIdentifierWords to the external
// test packages under tests/. The repo keeps tests out of this package, so a
// differential test against the regex implementation this replaced needs a
// seam; splitIdentifierWords itself stays unexported.
func SplitIdentifierWordsForTest(name string) []string {
	return splitIdentifierWords(name)
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
