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
}

func NewFinding(sc Context, input FindingInput) core.Finding {
	normalizedPath := filepath.ToSlash(input.Path)
	meta := sc.RuleCatalog[input.RuleID]
	if input.Level == "" {
		input.Level = meta.DefaultLevel
	}
	input.Level = NormalizedSeverity(input.Level)
	sum := sha256.Sum256([]byte(strings.Join([]string{input.RuleID, normalizedPath, strconv.Itoa(input.Line), input.Message}, "|")))
	legacy := hex.EncodeToString(sum[:])
	contextFP := contextFingerprint(sc, input.RuleID, normalizedPath, input.Line)
	if contextFP == "" {
		contextFP = legacy
	}
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
		Fingerprint:        legacy,
		ContextFingerprint: contextFP,
	}
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
	section := core.SectionResult{ID: id, Name: name, Status: core.StatusPass}
	active := make([]core.Finding, 0, len(findings))
	for _, finding := range findings {
		if sc.Opts.Mode == core.ScanModeDiff && finding.Path != "" && !matchesDiff(sc, finding) {
			continue
		}
		if suppressed, reason := IsSuppressed(sc, finding); suppressed {
			section.SuppressedCount++
			sc.RuleStats.RecordSuppressed(finding.RuleID, reason)
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
