package runner

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type inlineSuppression struct {
	ruleID  string
	line    int
	expires string
}

var inlineIgnorePattern = regexp.MustCompile(`codeguard:ignore\s+([a-z0-9._*\-]+)(?:\s+until\s+(\d{4}-\d{2}-\d{2}))?`)

func (sc scanContext) isSuppressed(finding core.Finding) (bool, string) {
	if sc.baseline != nil {
		if _, ok := sc.baseline[finding.Fingerprint]; ok {
			return true, "baseline"
		}
	}
	if waiverMatches(sc, finding) {
		return true, "waiver"
	}
	fullPath := findingFullPath(sc, finding.Path)
	if fullPath == "" {
		return false, ""
	}
	directives, err := parseInlineSuppressions(fullPath)
	if err != nil {
		return false, ""
	}
	if inlineSuppressionMatches(sc, finding, directives) {
		return true, "inline suppression"
	}
	return false, ""
}

func waiverMatches(sc scanContext, finding core.Finding) bool {
	for _, waiver := range sc.cfg.Waivers {
		if waiver.Rule != "*" && waiver.Rule != finding.RuleID {
			continue
		}
		if waiver.Path != "" && !matchPattern(waiver.Path, finding.Path) {
			continue
		}
		if suppressionExpired(waiver.ExpiresOn, sc.today) {
			continue
		}
		return true
	}
	return false
}

func findingFullPath(sc scanContext, rel string) string {
	if rel == "" {
		return ""
	}
	for _, target := range sc.cfg.Targets {
		candidate := filepath.Join(target.Path, filepath.FromSlash(rel))
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

func inlineSuppressionMatches(sc scanContext, finding core.Finding, directives []inlineSuppression) bool {
	for _, directive := range directives {
		if directive.ruleID != "*" && directive.ruleID != finding.RuleID {
			continue
		}
		if suppressionExpired(directive.expires, sc.today) {
			continue
		}
		if finding.Line == 0 || finding.Line == directive.line || finding.Line == directive.line+1 {
			return true
		}
	}
	return false
}

func suppressionExpired(expires string, today time.Time) bool {
	if expires == "" {
		return false
	}
	parsed, err := time.Parse("2006-01-02", expires)
	return err == nil && parsed.Before(dateOnly(today))
}

func parseInlineSuppressions(path string) ([]inlineSuppression, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	out := make([]inlineSuppression, 0)
	for idx, line := range lines {
		matches := inlineIgnorePattern.FindAllStringSubmatch(line, -1)
		for _, match := range matches {
			out = append(out, inlineSuppression{
				ruleID:  match[1],
				line:    idx + 1,
				expires: match[2],
			})
		}
	}
	return out, nil
}
