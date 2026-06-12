package support

import (
	"crypto/sha1"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type FindingInput struct {
	RuleID  string
	Level   string
	Path    string
	Line    int
	Column  int
	Message string
	Why     string
}

type fileScanInput struct {
	sectionID string
	target    core.TargetConfig
	rel       string
	data      []byte
}

func ScanTargetFiles(sc Context, target core.TargetConfig, sectionID string, include func(string) bool, evaluator func(string, []byte) []core.Finding) []core.Finding {
	files, _ := WalkFiles(target.Path, sc.Cfg.Exclude, include)
	findings := make([]core.Finding, 0)
	for _, file := range files {
		data, err := os.ReadFile(filepath.Join(target.Path, file))
		if err != nil {
			continue
		}
		findings = append(findings, cachedFileFindings(sc, fileScanInput{
			sectionID: sectionID,
			target:    target,
			rel:       file,
			data:      data,
		}, func() []core.Finding {
			return evaluator(file, data)
		})...)
	}
	return findings
}

func cachedFileFindings(sc Context, input fileScanInput, compute func() []core.Finding) []core.Finding {
	if sc.Cache == nil {
		return compute()
	}
	key := cacheKey(input.sectionID, input.target.Path, input.rel)
	fileHash := hashBytes(input.data)
	if entry, ok := sc.Cache.entries[key]; ok && entry.FileHash == fileHash && entry.ConfigHash == sc.ConfigHash {
		return cloneFindings(entry.Findings)
	}
	findings := compute()
	sc.Cache.entries[key] = cacheEntry{
		FileHash:   fileHash,
		ConfigHash: sc.ConfigHash,
		Findings:   cloneFindings(findings),
	}
	sc.Cache.dirty = true
	return findings
}

func NewFinding(sc Context, input FindingInput) core.Finding {
	normalizedPath := filepath.ToSlash(input.Path)
	meta := sc.RuleCatalog[input.RuleID]
	if input.Level == "" {
		input.Level = meta.DefaultLevel
	}
	input.Level = NormalizedSeverity(input.Level)
	sum := sha1.Sum([]byte(strings.Join([]string{input.RuleID, normalizedPath, strconv.Itoa(input.Line), input.Message}, "|")))
	return core.Finding{
		RuleID:      input.RuleID,
		Level:       input.Level,
		Severity:    input.Level,
		Title:       meta.Title,
		Section:     meta.Section,
		Message:     input.Message,
		Why:         firstNonEmpty(input.Why, input.Message),
		HowToFix:    meta.HowToFix,
		Path:        normalizedPath,
		Line:        input.Line,
		Column:      input.Column,
		Fingerprint: hex.EncodeToString(sum[:]),
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
		if suppressed, _ := IsSuppressed(sc, finding); suppressed {
			section.SuppressedCount++
			continue
		}
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
