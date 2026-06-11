package codeguard

import (
	"github.com/devr-tools/codeguard/codeguard/core"
	"github.com/devr-tools/codeguard/codeguard/runner"
)

type (
	Config              = core.Config
	ScanMode            = core.ScanMode
	ScanOptions         = core.ScanOptions
	TargetConfig        = core.TargetConfig
	CheckConfig         = core.CheckConfig
	OutputConfig        = core.OutputConfig
	QualityRulesConfig  = core.QualityRulesConfig
	DesignRulesConfig   = core.DesignRulesConfig
	PromptRulesConfig   = core.PromptRulesConfig
	CIRulesConfig       = core.CIRulesConfig
	SecurityRulesConfig = core.SecurityRulesConfig
	WorkflowRuleConfig  = core.WorkflowRuleConfig
	Report              = core.Report
	SectionResult       = core.SectionResult
	Finding             = core.Finding
	Runner              = runner.Runner
)

const (
	ScanModeFull = core.ScanModeFull
	ScanModeDiff = core.ScanModeDiff
)
