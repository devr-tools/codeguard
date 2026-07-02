package codeguard

import (
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/internal/codeguard/runner"
)

type ReportSummary = core.ReportSummary
type RuleMetadata = core.RuleMetadata
type RuleExecutionModel = core.RuleExecutionModel
type RuleLanguage = core.RuleLanguage
type RuleLanguageCoverage = core.RuleLanguageCoverage
type RuleLanguageCoverageMode = core.RuleLanguageCoverageMode
type FixTemplate = core.FixTemplate
type FixTemplateKind = core.FixTemplateKind
type PolicyProfile = core.PolicyProfile
type Runner = runner.Runner

const (
	RuleExecutionModelGoNative         = core.RuleExecutionModelGoNative
	RuleExecutionModelLanguageAgnostic = core.RuleExecutionModelLanguageAgnostic
	RuleExecutionModelCommandDriven    = core.RuleExecutionModelCommandDriven
	RuleLanguageGo                     = core.RuleLanguageGo
	RuleLanguagePython                 = core.RuleLanguagePython
	RuleLanguageTypeScript             = core.RuleLanguageTypeScript
	RuleLanguageJavaScript             = core.RuleLanguageJavaScript
	RuleLanguageRust                   = core.RuleLanguageRust
	RuleLanguageJava                   = core.RuleLanguageJava
	RuleLanguageCSharp                 = core.RuleLanguageCSharp
	RuleLanguageRuby                   = core.RuleLanguageRuby
	RuleLanguageCoverageFixed          = core.RuleLanguageCoverageFixed
	RuleLanguageCoverageRepositoryWide = core.RuleLanguageCoverageRepositoryWide
	RuleLanguageCoverageConfigurable   = core.RuleLanguageCoverageConfigurable
	FixTemplateKindDeterministic       = core.FixTemplateKindDeterministic
	FixTemplateKindGuided              = core.FixTemplateKindGuided
	ScanModeFull                       = core.ScanModeFull
	ScanModeDiff                       = core.ScanModeDiff
	StatusPass                         = core.StatusPass
	StatusWarn                         = core.StatusWarn
	StatusFail                         = core.StatusFail
)
