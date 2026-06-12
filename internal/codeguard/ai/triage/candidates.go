package triage

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func collectCandidates(cfg core.Config, sections []core.SectionResult) []candidate {
	resolver := newSourceResolver(cfg.Targets)
	candidates := make([]candidate, 0)
	for sectionIndex, section := range sections {
		for findingIndex, finding := range section.Findings {
			item := candidate{
				sectionIndex: sectionIndex,
				findingIndex: findingIndex,
				sectionName:  section.Name,
				finding:      finding,
				snippet:      resolver.snippetForFinding(finding),
			}
			item.hash = contentHash(item)
			candidates = append(candidates, item)
		}
	}
	return candidates
}

func contentHash(item candidate) string {
	payload := map[string]any{
		"section":    item.sectionName,
		"rule_id":    item.finding.RuleID,
		"level":      item.finding.Level,
		"severity":   item.finding.Severity,
		"title":      item.finding.Title,
		"message":    item.finding.Message,
		"why":        item.finding.Why,
		"how_to_fix": item.finding.HowToFix,
		"path":       item.finding.Path,
		"line":       item.finding.Line,
		"column":     item.finding.Column,
		"snippet":    item.snippet,
	}
	data, _ := json.Marshal(payload)
	sum := sha1.Sum(data)
	return hex.EncodeToString(sum[:])
}

type sourceResolver struct {
	paths map[string]string
}

func newSourceResolver(targets []core.TargetConfig) sourceResolver {
	paths := make(map[string]string)
	for _, target := range targets {
		root := filepath.Clean(target.Path)
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() {
				return nil
			}
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return nil
			}
			rel = filepath.ToSlash(rel)
			if _, exists := paths[rel]; !exists {
				paths[rel] = path
			}
			return nil
		})
	}
	return sourceResolver{paths: paths}
}

func (resolver sourceResolver) snippetForFinding(finding core.Finding) string {
	if resolver.paths == nil || strings.TrimSpace(finding.Path) == "" {
		return ""
	}
	path, ok := resolver.paths[filepath.ToSlash(finding.Path)]
	if !ok {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if finding.Line <= 0 || finding.Line > len(lines) {
		return strings.TrimSpace(strings.Join(lines[:minInt(8, len(lines))], "\n"))
	}
	start := maxInt(0, finding.Line-4)
	end := minInt(len(lines), finding.Line+3)
	window := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		window = append(window, fmt.Sprintf("%d: %s", i+1, lines[i]))
	}
	return strings.TrimSpace(strings.Join(window, "\n"))
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
