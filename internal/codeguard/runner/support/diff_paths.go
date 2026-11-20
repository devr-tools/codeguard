package support

import (
	"strconv"
	"strings"
)

// diffHeaderPath returns the path named by a unified-diff "--- " or "+++ "
// header, given the text after the marker, still carrying its a/ or b/
// prefix (or "/dev/null").
//
// Git writes a name containing a space with a trailing tab ("+++ b/sp ace.go\t")
// and C-quotes a name containing non-ASCII bytes, quotes or control characters
// under the default core.quotePath ("+++ \"b/caf\\303\\251.go\""); other diff
// tools append "\t<timestamp>". Matching the raw header text instead dropped
// such files from the diff scope, so their findings were filtered out of a PR
// gate.
func diffHeaderPath(rest string) string {
	rest = strings.TrimRight(rest, "\r\n")
	if strings.HasPrefix(rest, `"`) {
		if unquoted, ok := unquoteGitPath(rest); ok {
			return unquoted
		}
	}
	if tab := strings.IndexByte(rest, '\t'); tab >= 0 {
		rest = rest[:tab]
	}
	return rest
}

// unquoteGitPath decodes a leading C-style quoted git path ("..."), returning
// false when the text is not a well-formed quoted string.
func unquoteGitPath(text string) (string, bool) {
	end := closingQuote(text)
	if end < 0 {
		return "", false
	}
	unquoted, err := strconv.Unquote(text[:end+1])
	if err != nil {
		return "", false
	}
	return unquoted, true
}

// closingQuote returns the index of the quote that closes the string opened at
// text[0], skipping backslash escapes, or -1.
func closingQuote(text string) int {
	for i := 1; i < len(text); i++ {
		switch text[i] {
		case '\\':
			i++
		case '"':
			return i
		}
	}
	return -1
}

// diffHeaderSidePath returns the header path with the given side prefix
// ("a/" or "b/") removed, and false when the header names another path (such
// as /dev/null).
func diffHeaderSidePath(rest string, side string) (string, bool) {
	path := diffHeaderPath(rest)
	if !strings.HasPrefix(path, side) {
		return "", false
	}
	return strings.TrimPrefix(path, side), true
}
