package report

import (
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// windowsDrivePath matches a slash-normalized Windows absolute path such as
// "C:/src/app.go", which filepath.IsAbs does not recognize on other platforms.
var windowsDrivePath = regexp.MustCompile(`^[A-Za-z]:/`)

// sarifArtifactURI turns a report path into a SARIF artifactLocation.uri.
// SARIF requires an RFC 3986 URI reference, so each path segment is
// percent-encoded while the separators are kept. Findings carry
// target-relative paths, but some diagnostics carry absolute filesystem paths
// (a target root, a Go module directory, a history file). An absolute path
// under the working directory becomes relative to it, because GitHub code
// scanning resolves relative URIs against the repository root and the Action
// runs from the checkout; any other absolute path becomes a file:// URI.
func sarifArtifactURI(path string) string {
	slashed := filepath.ToSlash(path)
	if !isAbsoluteReportPath(slashed) {
		return encodeURIPath(slashed)
	}
	if rel, ok := relativeToWorkingDir(path); ok {
		return encodeURIPath(rel)
	}
	if windowsDrivePath.MatchString(slashed) {
		// The drive letter's colon is legal in a path segment of a URI with
		// a scheme, so only the remainder is encoded.
		return "file:///" + slashed[:2] + encodeURIPath(slashed[2:])
	}
	return "file://" + encodeURIPath(slashed)
}

func isAbsoluteReportPath(slashed string) bool {
	return strings.HasPrefix(slashed, "/") || windowsDrivePath.MatchString(slashed) || filepath.IsAbs(slashed)
}

func relativeToWorkingDir(path string) (string, bool) {
	wd, err := os.Getwd()
	if err != nil {
		return "", false
	}
	rel, err := filepath.Rel(wd, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

// encodeURIPath percent-encodes every segment of a slash-separated path. A
// colon is encoded too: in a relative reference a colon in the first segment
// would otherwise be read as a URI scheme.
func encodeURIPath(slashed string) string {
	segments := strings.Split(slashed, "/")
	for i, segment := range segments {
		segments[i] = strings.ReplaceAll(url.PathEscape(segment), ":", "%3A")
	}
	return strings.Join(segments, "/")
}
