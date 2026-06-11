package codeguard

import corepkg "github.com/devr-tools/codeguard/codeguard"

type (
	Config              = corepkg.Config
	ScanMode            = corepkg.ScanMode
	ScanOptions         = corepkg.ScanOptions
	TargetConfig        = corepkg.TargetConfig
	CheckConfig         = corepkg.CheckConfig
	OutputConfig        = corepkg.OutputConfig
	QualityRulesConfig  = corepkg.QualityRulesConfig
	DesignRulesConfig   = corepkg.DesignRulesConfig
	PromptRulesConfig   = corepkg.PromptRulesConfig
	CIRulesConfig       = corepkg.CIRulesConfig
	SecurityRulesConfig = corepkg.SecurityRulesConfig
	WorkflowRuleConfig  = corepkg.WorkflowRuleConfig
	Report              = corepkg.Report
	SectionResult       = corepkg.SectionResult
	Finding             = corepkg.Finding
	Runner              = corepkg.Runner
)

const (
	ScanModeFull = corepkg.ScanModeFull
	ScanModeDiff = corepkg.ScanModeDiff
)
