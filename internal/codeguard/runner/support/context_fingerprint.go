package support

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// contextFingerprintRadius is how many source lines on each side of the
// finding line are folded into the context fingerprint.
const contextFingerprintRadius = 2

// contextFingerprint returns sha256(ruleID|path|normalizedContext), where the
// normalized context is the finding's source line plus up to
// contextFingerprintRadius lines on each side, each with runs of whitespace
// collapsed to a single space and trimmed. Unlike the legacy fingerprint it
// does not embed the line number, so unrelated edits that merely shift the
// finding up or down the file leave it unchanged. It returns "" when no source
// context is available (line <= 0, the path resolves under no target, the file
// is unreadable, or the line is past end of file), letting the caller fall
// back to the legacy fingerprint.
func contextFingerprint(sc Context, ruleID string, normalizedPath string, line int) string {
	if line <= 0 || normalizedPath == "" {
		return ""
	}
	data, ok := findingSource(sc, normalizedPath)
	if !ok {
		return ""
	}
	context, ok := normalizedContext(data, line)
	if !ok {
		return ""
	}
	sum := sha256.Sum256([]byte(strings.Join([]string{ruleID, normalizedPath, context}, "|")))
	return hex.EncodeToString(sum[:])
}

// findingSource resolves a finding's target-relative path against the scan
// targets (mirroring findingFullPath) and reads it through the per-scan
// corpus, so the bytes already loaded for scanning are reused rather than
// re-read from disk.
func findingSource(sc Context, rel string) ([]byte, bool) {
	for _, target := range sc.Cfg.Targets {
		data, err := sc.corpusRead(target.Path, rel)
		if err != nil {
			continue
		}
		return data, true
	}
	return nil, false
}

// normalizedContext extracts the whitespace-normalized window of source lines
// around the (1-based) finding line. The window is clamped to the file bounds,
// so a finding on the first or last line simply carries a smaller context.
func normalizedContext(data []byte, line int) (string, bool) {
	lines := strings.Split(string(data), "\n")
	if line > len(lines) {
		return "", false
	}
	start := max(1, line-contextFingerprintRadius)
	end := min(len(lines), line+contextFingerprintRadius)
	window := make([]string, 0, end-start+1)
	for i := start; i <= end; i++ {
		window = append(window, strings.Join(strings.Fields(lines[i-1]), " "))
	}
	return strings.Join(window, "\n"), true
}
