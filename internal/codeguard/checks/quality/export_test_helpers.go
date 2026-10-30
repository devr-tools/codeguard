package quality

// SplitIdentifierWordsForTest exposes splitIdentifierWords to the external
// test packages under tests/. The repo keeps tests out of this package, so a
// differential test against the regex implementation this replaced needs a
// seam; splitIdentifierWords itself stays unexported.
func SplitIdentifierWordsForTest(name string) []string {
	return splitIdentifierWords(name)
}
