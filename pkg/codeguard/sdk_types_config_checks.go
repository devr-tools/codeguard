package codeguard

import "github.com/devr-tools/codeguard/internal/codeguard/core"

type QualityRulesConfig = core.QualityRulesConfig
type QualityDeadCodeConfig = core.QualityDeadCodeConfig
type GoDeadCodeToolchainConfig = core.GoDeadCodeToolchainConfig
type RustDeadCodeConfig = core.RustDeadCodeConfig
type CPPDeadCodeConfig = core.CPPDeadCodeConfig
type PythonDeadCodeConfig = core.PythonDeadCodeConfig
type ScriptDeadCodeConfig = core.ScriptDeadCodeConfig
type QualityNamingConfig = core.QualityNamingConfig
type QualityNamingGlossaryEntry = core.QualityNamingGlossaryEntry
type CPPToolingConfig = core.CPPToolingConfig
type DesignRulesConfig = core.DesignRulesConfig
type PromptRulesConfig = core.PromptRulesConfig
type CIRulesConfig = core.CIRulesConfig
type DeliveryRulesConfig = core.DeliveryRulesConfig
type SupplyChainRulesConfig = core.SupplyChainRulesConfig
type ReliabilityRulesConfig = core.ReliabilityRulesConfig
type DataRulesConfig = core.DataRulesConfig
type ObservabilityRulesConfig = core.ObservabilityRulesConfig
type OperationsRulesConfig = core.OperationsRulesConfig
type ChangeRulesConfig = core.ChangeRulesConfig
type ProductionRiskConfig = core.ProductionRiskConfig
type ContractRulesConfig = core.ContractRulesConfig
type ContextRulesConfig = core.ContextRulesConfig

const (
	ExternalToolModeOff      = core.ExternalToolModeOff
	ExternalToolModeAuto     = core.ExternalToolModeAuto
	ExternalToolModeRequired = core.ExternalToolModeRequired
)

type WorkflowRuleConfig = core.WorkflowRuleConfig
type CommandCheckConfig = core.CommandCheckConfig
