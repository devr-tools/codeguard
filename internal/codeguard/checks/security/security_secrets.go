package security

import (
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// Bounds that keep the scan cheap and resistant to pathological (and untrusted)
// input such as minified bundles or deliberately oversized lines. codeguard runs
// on PR content it does not control, so these caps are a hardening measure as
// well as a performance one.
const (
	maxScanFileBytes = 5 << 20  // skip files larger than 5 MiB
	maxScanLineBytes = 64 << 10 // scan at most the first 64 KiB of any line
	binarySniffBytes = 8 << 10  // bytes inspected when detecting binary content
)

// ScanContent runs the secret/credential scan over file content and returns the
// matches with 1-based line numbers. Path allowlisting is the caller's
// responsibility (see SkipPath).
func (s Scanner) ScanContent(content string) []Match {
	matches := make([]Match, 0)
	lineNo := 0
	start := 0
	for i := 0; i <= len(content); i++ {
		if i != len(content) && content[i] != '\n' {
			continue
		}
		lineNo++
		line := strings.TrimSuffix(content[start:i], "\r")
		start = i + 1
		if !s.lineAllowed(line) {
			matches = append(matches, s.scanLine(lineNo, line)...)
		}
	}
	return matches
}

// scanLine reports at most one match per line, preferring the highest-confidence
// tier: PEM key material and known/custom credential formats fail; the name-based
// heuristic warns; the optional entropy pass is last. Overlong lines are scanned
// only up to maxScanLineBytes to bound worst-case cost on minified/oversized input.
func (s Scanner) scanLine(lineNo int, line string) []Match {
	if len(line) > maxScanLineBytes {
		line = line[:maxScanLineBytes]
	}
	// Cheap literal gate: when no built-in marker is present, the expensive
	// per-pattern regexes are skipped. This gates built-ins even in entropy mode
	// (the gate is a superset of what they can match), so entropy only adds its
	// own literal pass. Custom patterns have arbitrary markers and always run.
	runBuiltins := builtinGatePasses(line)
	if runBuiltins {
		if m := matchPrivateKey(line); m != nil {
			return located(m, lineNo)
		}
		if m := matchCredential(line); m != nil {
			return located(m, lineNo)
		}
	}
	if m := s.matchCustom(line); m != nil {
		return located(m, lineNo)
	}
	if runBuiltins {
		if m := matchNameBased(line); m != nil {
			return located(m, lineNo)
		}
	}
	if s.entropy.enabled {
		if m := s.entropyMatch(lineNo, line); m != nil {
			return []Match{*m}
		}
	}
	return nil
}

// located stamps the line/column onto a match and wraps it as a single-element
// result, so the tier helpers can stay position-agnostic.
func located(m *Match, lineNo int) []Match {
	m.Line = lineNo
	m.Column = 1
	return []Match{*m}
}

// secretFindingsForFile runs the scan over a single file and converts matches to
// findings. It applies the path allowlist, skips binary/oversized files, and
// demotes fixture-path matches when the demotion toggle is on.
func secretFindingsForFile(env support.Context, file string, data []byte, scanner Scanner) []core.Finding {
	if scanner.SkipPath(file) || len(data) > maxScanFileBytes || looksBinary(data) {
		return nil
	}
	demote := fixtureDemotionEnabled(env.Config.Checks.SecurityRules) && isFixturePath(file)
	matches := scanner.ScanContent(string(data))
	findings := make([]core.Finding, 0, len(matches))
	for _, match := range matches {
		if demote {
			match = demoteFixtureMatch(match)
		}
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:     match.RuleID,
			Level:      match.Level,
			Path:       file,
			Line:       match.Line,
			Column:     match.Column,
			Message:    match.Message,
			Confidence: match.Confidence,
			Metadata:   secretMetadata(match),
		}))
	}
	return findings
}

func secretMetadata(match Match) map[string]string {
	if match.SecretType == "" {
		return nil
	}
	return map[string]string{"secret_type": match.SecretType}
}
