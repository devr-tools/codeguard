package runner

import (
	"crypto/sha1"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type findingInput struct {
	ruleID  string
	level   string
	path    string
	line    int
	column  int
	message string
}

type fileScanInput struct {
	sectionID string
	target    core.TargetConfig
	rel       string
	data      []byte
}

func scanTargetFiles(sc scanContext, target core.TargetConfig, sectionID string, include func(string) bool, evaluator func(string, []byte) []core.Finding) []core.Finding {
	files, _ := walkFiles(target.Path, sc.cfg.Exclude, include)
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

func cachedFileFindings(sc scanContext, input fileScanInput, compute func() []core.Finding) []core.Finding {
	if sc.cache == nil {
		return compute()
	}
	key := cacheKey(input.sectionID, input.target.Path, input.rel)
	fileHash := hashBytes(input.data)
	if entry, ok := sc.cache.entries[key]; ok && entry.FileHash == fileHash && entry.ConfigHash == sc.configHash {
		return cloneFindings(entry.Findings)
	}
	findings := compute()
	sc.cache.entries[key] = cacheEntry{
		FileHash:   fileHash,
		ConfigHash: sc.configHash,
		Findings:   cloneFindings(findings),
	}
	sc.cache.dirty = true
	return findings
}

func newFinding(sc scanContext, input findingInput) core.Finding {
	normalizedPath := filepath.ToSlash(input.path)
	meta := sc.ruleCatalog[input.ruleID]
	if input.level == "" {
		input.level = meta.DefaultLevel
	}
	input.level = normalizedSeverity(input.level)
	sum := sha1.Sum([]byte(strings.Join([]string{input.ruleID, normalizedPath, strconv.Itoa(input.line), input.message}, "|")))
	return core.Finding{
		RuleID:      input.ruleID,
		Level:       input.level,
		Severity:    input.level,
		Title:       meta.Title,
		Section:     meta.Section,
		Message:     input.message,
		Why:         input.message,
		HowToFix:    meta.HowToFix,
		Path:        normalizedPath,
		Line:        input.line,
		Column:      input.column,
		Fingerprint: hex.EncodeToString(sum[:]),
	}
}

func finalizeSection(sc scanContext, id string, name string, findings []core.Finding) core.SectionResult {
	section := core.SectionResult{ID: id, Name: name, Status: core.StatusPass}
	active := make([]core.Finding, 0, len(findings))
	for _, finding := range findings {
		if sc.opts.Mode == core.ScanModeDiff && finding.Path != "" && !matchesDiff(sc, finding) {
			continue
		}
		if suppressed, _ := sc.isSuppressed(finding); suppressed {
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

func matchesDiff(sc scanContext, finding core.Finding) bool {
	scope, ok := sc.diff[finding.Path]
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

func isPromptFile(sc scanContext, rel string) bool {
	rel = filepath.ToSlash(rel)
	ext := strings.ToLower(filepath.Ext(rel))
	for _, allowed := range sc.cfg.Checks.PromptRules.FileExtensions {
		if strings.EqualFold(ext, allowed) {
			for _, token := range sc.cfg.Checks.PromptRules.PathContains {
				if strings.Contains(strings.ToLower(rel), strings.ToLower(token)) {
					return true
				}
			}
		}
	}
	return false
}
