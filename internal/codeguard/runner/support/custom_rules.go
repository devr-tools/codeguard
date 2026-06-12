package support

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type CompiledCustomRule struct {
	PackName     string
	Rule         core.CustomRuleConfig
	PathRegex    *regexp.Regexp
	ContentRegex *regexp.Regexp
}

func compileCustomRules(cfg core.Config) ([]CompiledCustomRule, error) {
	out := make([]CompiledCustomRule, 0)
	for _, pack := range cfg.RulePacks {
		for _, rule := range pack.Rules {
			compiled := CompiledCustomRule{
				PackName: pack.Name,
				Rule:     rule,
			}
			if strings.TrimSpace(rule.PathRegex) != "" {
				re, err := regexp.Compile(rule.PathRegex)
				if err != nil {
					return nil, err
				}
				compiled.PathRegex = re
			}
			if strings.TrimSpace(rule.ContentRegex) != "" {
				re, err := regexp.Compile(rule.ContentRegex)
				if err != nil {
					return nil, err
				}
				compiled.ContentRegex = re
			}
			out = append(out, compiled)
		}
	}
	return out, nil
}

func (rule CompiledCustomRule) MatchesPath(rel string) bool {
	rel = filepath.ToSlash(rel)
	return rule.matchesExclude(rel) && rule.matchesExtensions(rel) && rule.matchesIncludedPaths(rel) && rule.matchesRegex(rel)
}

func (rule CompiledCustomRule) UsesNaturalLanguage() bool {
	return strings.TrimSpace(rule.Rule.NaturalLanguage) != ""
}

func (rule CompiledCustomRule) matchesExclude(rel string) bool {
	for _, excluded := range rule.Rule.Exclude {
		if MatchPattern(excluded, rel) {
			return false
		}
	}
	return true
}

func (rule CompiledCustomRule) matchesExtensions(rel string) bool {
	if len(rule.Rule.FileExtensions) == 0 {
		return true
	}
	ext := strings.ToLower(filepath.Ext(rel))
	for _, allowed := range rule.Rule.FileExtensions {
		if strings.EqualFold(ext, allowed) {
			return true
		}
	}
	return false
}

func (rule CompiledCustomRule) matchesIncludedPaths(rel string) bool {
	if len(rule.Rule.Paths) == 0 {
		return true
	}
	for _, pattern := range rule.Rule.Paths {
		if MatchPattern(pattern, rel) {
			return true
		}
	}
	return false
}

func (rule CompiledCustomRule) matchesRegex(rel string) bool {
	return rule.PathRegex == nil || rule.PathRegex.MatchString(rel)
}
