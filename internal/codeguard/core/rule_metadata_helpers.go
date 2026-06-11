package core

import (
	"sort"
	"strings"
)

func FixedRuleLanguageCoverage(languages ...RuleLanguage) RuleLanguageCoverage {
	return RuleLanguageCoverage{
		Mode:      RuleLanguageCoverageFixed,
		Languages: append([]RuleLanguage(nil), languages...),
	}
}

func RepositoryWideRuleLanguageCoverage() RuleLanguageCoverage {
	return RuleLanguageCoverage{Mode: RuleLanguageCoverageRepositoryWide}
}

func ConfigurableRuleLanguageCoverage() RuleLanguageCoverage {
	return RuleLanguageCoverage{Mode: RuleLanguageCoverageConfigurable}
}

func (coverage RuleLanguageCoverage) String() string {
	coverage = normalizeRuleLanguageCoverage(coverage)
	switch coverage.Mode {
	case RuleLanguageCoverageFixed:
		parts := make([]string, 0, len(coverage.Languages))
		for _, language := range coverage.Languages {
			parts = append(parts, string(language))
		}
		return strings.Join(parts, ", ")
	case RuleLanguageCoverageRepositoryWide:
		return "repository-wide"
	case RuleLanguageCoverageConfigurable:
		return "configurable"
	default:
		return ""
	}
}

func NormalizeRuleMetadata(meta RuleMetadata) RuleMetadata {
	meta.LanguageCoverage = normalizeRuleLanguageCoverage(meta.LanguageCoverage)
	if meta.LanguageCoverage.Mode == "" {
		meta.LanguageCoverage = defaultRuleLanguageCoverage(meta.ID, meta.ExecutionModel)
	}
	return meta
}

func defaultRuleLanguageCoverage(ruleID string, executionModel RuleExecutionModel) RuleLanguageCoverage {
	if language, ok := ruleLanguageFromRuleID(ruleID); ok {
		return FixedRuleLanguageCoverage(language)
	}

	switch strings.TrimSpace(ruleID) {
	case
		"quality.max-file-lines",
		"quality.max-function-lines",
		"quality.max-parameters",
		"quality.cyclomatic-complexity",
		"ci.test-file-location":
		return FixedRuleLanguageCoverage(RuleLanguageGo, RuleLanguagePython, RuleLanguageTypeScript)
	case "quality.command-check", "security.command-check":
		return ConfigurableRuleLanguageCoverage()
	case
		"security.hardcoded-secret",
		"security.private-key",
		"prompts.secret-interpolation",
		"prompts.unsafe-instructions",
		"ci.required-workflow-dir",
		"ci.required-file",
		"ci.workflow-content":
		return RepositoryWideRuleLanguageCoverage()
	case
		"security.insecure-tls",
		"security.shell-execution",
		"security.govulncheck":
		return FixedRuleLanguageCoverage(RuleLanguageGo)
	}

	switch executionModel {
	case RuleExecutionModelGoNative:
		return FixedRuleLanguageCoverage(RuleLanguageGo)
	case RuleExecutionModelCommandDriven:
		return ConfigurableRuleLanguageCoverage()
	default:
		return RepositoryWideRuleLanguageCoverage()
	}
}

func normalizeRuleLanguageCoverage(coverage RuleLanguageCoverage) RuleLanguageCoverage {
	switch coverage.Mode {
	case RuleLanguageCoverageFixed:
		if len(coverage.Languages) == 0 {
			return RepositoryWideRuleLanguageCoverage()
		}

		seen := make(map[RuleLanguage]struct{}, len(coverage.Languages))
		languages := make([]RuleLanguage, 0, len(coverage.Languages))
		for _, language := range coverage.Languages {
			canonical := canonicalRuleLanguage(language)
			if canonical == "" {
				continue
			}
			if _, ok := seen[canonical]; ok {
				continue
			}
			seen[canonical] = struct{}{}
			languages = append(languages, canonical)
		}
		sort.Slice(languages, func(i, j int) bool { return languages[i] < languages[j] })
		if len(languages) == 0 {
			return RepositoryWideRuleLanguageCoverage()
		}
		coverage.Languages = languages
		return coverage
	case RuleLanguageCoverageRepositoryWide, RuleLanguageCoverageConfigurable:
		coverage.Languages = nil
		return coverage
	default:
		return RuleLanguageCoverage{}
	}
}

func ruleLanguageFromRuleID(ruleID string) (RuleLanguage, bool) {
	parts := strings.Split(strings.TrimSpace(ruleID), ".")
	if len(parts) < 3 {
		return "", false
	}
	language := canonicalRuleLanguage(RuleLanguage(parts[1]))
	return language, language != ""
}

func canonicalRuleLanguage(language RuleLanguage) RuleLanguage {
	switch strings.ToLower(strings.TrimSpace(string(language))) {
	case "go", "golang":
		return RuleLanguageGo
	case "python", "py":
		return RuleLanguagePython
	case "typescript", "javascript", "ts", "tsx", "js", "jsx":
		return RuleLanguageTypeScript
	default:
		return ""
	}
}
