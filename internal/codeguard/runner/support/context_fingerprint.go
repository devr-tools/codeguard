package support

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// contextFingerprintRadius is how many source lines on each side of the
// finding line are folded into the context fingerprint.
const contextFingerprintRadius = 2

// contextFingerprint returns sha256(ruleID|path|normalizedContext), where the
// normalized context is the finding's source line plus up to
// contextFingerprintRadius lines on each side, each with runs of whitespace
// collapsed to a single space and trimmed. Unlike the exact fingerprint it
// does not embed the line number, so unrelated edits that merely shift the
// finding up or down the file leave it unchanged. It returns "" when no source
// context is available (line <= 0, the path resolves under no target, the file
// is unreadable, or the line is past end of file), letting the caller fall
// back to the exact fingerprint.
func contextFingerprint(sc Context, root string, ruleID string, normalizedPath string, line int) string {
	return findingContextFingerprint(sc, root, ruleID, normalizedPath, line, true)
}

func contentFingerprint(sc Context, root string, ruleID string, normalizedPath string, line int) string {
	return findingContextFingerprint(sc, root, ruleID, normalizedPath, line, false)
}

// sourceIdentity returns the normalized source line at a finding location.
// It is intentionally narrower than contextFingerprint: exact identity keeps
// its line coordinate while remaining independent of diagnostic prose.
func sourceIdentity(sc Context, root string, normalizedPath string, line int) string {
	if line <= 0 || normalizedPath == "" {
		return ""
	}
	data, ok := findingSource(sc, root, normalizedPath)
	if !ok {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	if line > len(lines) {
		return ""
	}
	return strings.Join(strings.Fields(lines[line-1]), " ")
}

func findingContextFingerprint(sc Context, root string, ruleID string, normalizedPath string, line int, includePath bool) string {
	if line <= 0 || normalizedPath == "" {
		return ""
	}
	data, ok := findingSource(sc, root, normalizedPath)
	if !ok {
		return ""
	}
	context, ok := normalizedContext(data, line)
	if !ok {
		return ""
	}
	parts := []string{ruleID, context}
	if includePath {
		parts = []string{ruleID, normalizedPath, context}
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:])
}

// findingSource reads a finding's target-relative path through the per-scan
// corpus, so the bytes already loaded for scanning are reused rather than
// re-read from disk. root is the finding's own target when known; when empty
// the path resolves against the first target that has it.
func findingSource(sc Context, root string, rel string) ([]byte, bool) {
	if root != "" {
		data, err := sc.corpusRead(root, rel)
		return data, err == nil
	}
	for _, target := range sc.Cfg.Targets {
		data, err := sc.corpusRead(target.Path, rel)
		if err != nil {
			continue
		}
		return data, true
	}
	return nil, false
}

// findingIdentity computes a finding's exact, context and content
// fingerprints from the source under root ("" for the first target that has
// the path).
func findingIdentity(sc Context, root string, ruleID string, normalizedPath string, line int) (string, string, string) {
	// Exact identity deliberately excludes diagnostic prose and evidence. The
	// version prefix makes future identity-shape migrations explicit, while the
	// normalized source line distinguishes multiple findings at the same
	// rule/path/line without coupling identity to Message or Metadata.
	sum := sha256.Sum256([]byte(strings.Join([]string{
		"codeguard-finding/v2",
		ruleID,
		normalizedPath,
		strconv.Itoa(line),
		sourceIdentity(sc, root, normalizedPath, line),
	}, "|")))
	exact := hex.EncodeToString(sum[:])
	contextFP := contextFingerprint(sc, root, ruleID, normalizedPath, line)
	if contextFP == "" {
		contextFP = exact
	}
	return exact, contextFP, contentFingerprint(sc, root, ruleID, normalizedPath, line)
}

// bindFindingsToTarget records the target that produced each finding. When a
// finding's relative path also exists under another target, its fingerprints
// were computed (in NewFinding) from the first such target's file, so they are
// recomputed from its own target's source. Unambiguous findings keep their
// fingerprints unchanged, so existing baselines still match them.
func bindFindingsToTarget(sc Context, target core.TargetConfig, findings []core.Finding) {
	ambiguous := map[string]bool{}
	for i := range findings {
		finding := &findings[i]
		if finding.Path == "" || finding.TargetPath != "" {
			continue
		}
		finding.TargetPath = target.Path
		if len(sc.Cfg.Targets) < 2 {
			continue
		}
		isAmbiguous, seen := ambiguous[finding.Path]
		if !seen {
			isAmbiguous = len(findingTargetPaths(sc, finding.Path)) > 1
			ambiguous[finding.Path] = isAmbiguous
		}
		if isAmbiguous {
			finding.Fingerprint, finding.ContextFingerprint, finding.ContentFingerprint =
				findingIdentity(sc, target.Path, finding.RuleID, finding.Path, finding.Line)
		}
	}
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
