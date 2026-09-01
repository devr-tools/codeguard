package support

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type FindingInput struct {
	RuleID     string
	Level      string
	Path       string
	Line       int
	Column     int
	Message    string
	Why        string
	Confidence string
	Metadata   map[string]string
}

func NewFinding(sc Context, input FindingInput) core.Finding {
	normalizedPath := filepath.ToSlash(input.Path)
	meta := sc.RuleCatalog[input.RuleID]
	if input.Level == "" {
		input.Level = meta.DefaultLevel
	}
	input.Level = NormalizedSeverity(input.Level)
	// Exact identity deliberately excludes diagnostic prose and evidence. The
	// version prefix makes future identity-shape migrations explicit, while the
	// normalized source line distinguishes multiple findings at the same
	// rule/path/line without coupling identity to Message or Metadata.
	sum := sha256.Sum256([]byte(strings.Join([]string{
		"codeguard-finding/v2",
		input.RuleID,
		normalizedPath,
		strconv.Itoa(input.Line),
		sourceIdentity(sc, normalizedPath, input.Line),
	}, "|")))
	exact := hex.EncodeToString(sum[:])
	contextFP := contextFingerprint(sc, input.RuleID, normalizedPath, input.Line)
	if contextFP == "" {
		contextFP = exact
	}
	contentFP := contentFingerprint(sc, input.RuleID, normalizedPath, input.Line)
	return core.Finding{
		RuleID:             input.RuleID,
		Level:              input.Level,
		Severity:           input.Level,
		Confidence:         core.NormalizedConfidence(input.Confidence),
		Title:              meta.Title,
		Section:            meta.Section,
		Message:            input.Message,
		Why:                firstNonEmpty(input.Why, input.Message),
		HowToFix:           meta.HowToFix,
		Path:               normalizedPath,
		Line:               input.Line,
		Column:             input.Column,
		Fingerprint:        exact,
		ContextFingerprint: contextFP,
		ContentFingerprint: contentFP,
		Metadata:           cloneMetadata(input.Metadata),
	}
}

// preV2ExactFingerprint reconstructs the message-based exact identity written
// before source-derived v2 fingerprints. It is used only as a baseline lookup
// key; new findings never expose or persist this compatibility identity.
func preV2ExactFingerprint(ruleID string, normalizedPath string, line int, message string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{ruleID, normalizedPath, strconv.Itoa(line), message}, "|")))
	return hex.EncodeToString(sum[:])
}

func cloneMetadata(metadata map[string]string) map[string]string {
	if len(metadata) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(metadata))
	for key, value := range metadata {
		cloned[key] = value
	}
	return cloned
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func FinalizeSection(sc Context, id string, name string, findings []core.Finding) core.SectionResult {
	return FinalizeSectionWithDiagnostics(sc, id, name, findings, nil)
}

func FinalizeSectionWithDiagnostics(sc Context, id string, name string, findings []core.Finding, diagnostics []core.Diagnostic) core.SectionResult {
	if sc.corpus != nil {
		diagnostics = append(diagnostics, sc.corpus.takeDiagnostics()...)
	}
	section := core.SectionResult{ID: id, Name: name, Status: core.StatusPass}
	active := make([]core.Finding, 0, len(findings))
	for _, finding := range findings {
		if sc.Opts.Mode == core.ScanModeDiff && finding.Path != "" && !matchesDiff(sc, finding) {
			continue
		}
		sc.WaiverAudit.RecordMatches(MatchingWaivers(sc, finding), finding)
		if suppression := MatchSuppression(sc, finding); suppression != nil {
			section.SuppressedCount++
			sc.RuleStats.RecordSuppressed(finding.RuleID, suppressionReason(suppression))
			if sc.Opts.IncludeSuppressed {
				finding.Suppressed = true
				finding.SuppressionReason = suppressionReason(suppression)
				finding.Suppression = suppression
				sc.Suppressed.Add(finding)
			}
			continue
		}
		sc.RuleStats.RecordEmitted(finding.RuleID)
		active = append(active, finding)
		switch finding.Level {
		case "fail":
			section.Status = core.StatusFail
		case "warn":
			if section.Status != core.StatusFail {
				section.Status = core.StatusWarn
			}
		}
	}
	section.Findings = active
	section.Diagnostics = diagnostics
	for _, diagnostic := range diagnostics {
		if !diagnostic.Operational {
			continue
		}
		switch diagnostic.Level {
		case "fail":
			section.Status = core.StatusFail
		case "warn":
			if section.Status != core.StatusFail {
				section.Status = core.StatusWarn
			}
		}
	}
	if sc.Opts.OnSectionComplete != nil {
		sc.Opts.OnSectionComplete(section)
	}
	return section
}

func matchesDiff(sc Context, finding core.Finding) bool {
	scope, ok := sc.Diff[finding.Path]
	if !ok {
		return false
	}
	if scope.allChanged || finding.Line <= 0 {
		return true
	}
	for _, r := range scope.ranges {
		if finding.Line >= r[0] && finding.Line <= r[1] {
			return true
		}
	}
	return false
}

func IsPromptFile(sc Context, rel string) bool {
	rel = filepath.ToSlash(rel)
	ext := strings.ToLower(filepath.Ext(rel))
	for _, allowed := range sc.Cfg.Checks.PromptRules.FileExtensions {
		if strings.EqualFold(ext, allowed) {
			for _, token := range sc.Cfg.Checks.PromptRules.PathContains {
				if strings.Contains(strings.ToLower(rel), strings.ToLower(token)) {
					return true
				}
			}
		}
	}
	return false
}
