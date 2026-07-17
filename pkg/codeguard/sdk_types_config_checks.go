package codeguard

import "github.com/devr-tools/codeguard/internal/codeguard/core"

type QualityRulesConfig = core.QualityRulesConfig
type CPPToolingConfig = core.CPPToolingConfig
type DesignRulesConfig = core.DesignRulesConfig
type PromptRulesConfig = core.PromptRulesConfig
type CIRulesConfig = core.CIRulesConfig
type SupplyChainRulesConfig = core.SupplyChainRulesConfig
type ContractRulesConfig = core.ContractRulesConfig
type ContextRulesConfig = core.ContextRulesConfig

const (
	ExternalToolModeOff      = core.ExternalToolModeOff
	ExternalToolModeAuto     = core.ExternalToolModeAuto
	ExternalToolModeRequired = core.ExternalToolModeRequired
)

type WorkflowRuleConfig = core.WorkflowRuleConfig
type CommandCheckConfig = core.CommandCheckConfig
