package runner

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type compiledCustomRule struct {
	packName     string
	rule         core.CustomRuleConfig
	pathRegex    *regexp.Regexp
	contentRegex *regexp.Regexp
}

func compileCustomRules(cfg core.Config) ([]compiledCustomRule, error) {
	out := make([]compiledCustomRule, 0)
	for _, pack := range cfg.RulePacks {
		for _, rule := range pack.Rules {
			compiled := compiledCustomRule{
				packName: pack.Name,
				rule:     rule,
			}
			if strings.TrimSpace(rule.PathRegex) != "" {
				re, err := regexp.Compile(rule.PathRegex)
				if err != nil {
					return nil, err
				}
				compiled.pathRegex = re
			}
			if strings.TrimSpace(rule.ContentRegex) != "" {
				re, err := regexp.Compile(rule.ContentRegex)
				if err != nil {
					return nil, err
				}
				compiled.contentRegex = re
			}
			out = append(out, compiled)
		}
	}
	return out, nil
}

func (rule compiledCustomRule) matchesPath(rel string) bool {
	rel = filepath.ToSlash(rel)
	return rule.matchesExclude(rel) && rule.matchesExtensions(rel) && rule.matchesIncludedPaths(rel) && rule.matchesRegex(rel)
}

func (rule compiledCustomRule) matchesExclude(rel string) bool {
	for _, excluded := range rule.rule.Exclude {
		if matchPattern(excluded, rel) {
			return false
		}
	}
	return true
}

func (rule compiledCustomRule) matchesExtensions(rel string) bool {
	if len(rule.rule.FileExtensions) == 0 {
		return true
	}
	ext := strings.ToLower(filepath.Ext(rel))
	for _, allowed := range rule.rule.FileExtensions {
		if strings.EqualFold(ext, allowed) {
			return true
		}
	}
	return false
}

func (rule compiledCustomRule) matchesIncludedPaths(rel string) bool {
	if len(rule.rule.Paths) == 0 {
		return true
	}
	for _, pattern := range rule.rule.Paths {
		if matchPattern(pattern, rel) {
			return true
		}
	}
	return false
}

func (rule compiledCustomRule) matchesRegex(rel string) bool {
	return rule.pathRegex == nil || rule.pathRegex.MatchString(rel)
}
