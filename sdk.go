package codeguard

import (
	"context"
	"io"

	corepkg "github.com/devr-tools/codeguard/codeguard"
)

type Config = corepkg.Config
type ScanMode = corepkg.ScanMode
type ScanOptions = corepkg.ScanOptions
type TargetConfig = corepkg.TargetConfig
type CheckConfig = corepkg.CheckConfig
type OutputConfig = corepkg.OutputConfig
type QualityRulesConfig = corepkg.QualityRulesConfig
type DesignRulesConfig = corepkg.DesignRulesConfig
type PromptRulesConfig = corepkg.PromptRulesConfig
type CIRulesConfig = corepkg.CIRulesConfig
type SecurityRulesConfig = corepkg.SecurityRulesConfig
type WorkflowRuleConfig = corepkg.WorkflowRuleConfig
type Report = corepkg.Report
type SectionResult = corepkg.SectionResult
type Finding = corepkg.Finding
type Runner = corepkg.Runner

const (
	ScanModeFull = corepkg.ScanModeFull
	ScanModeDiff = corepkg.ScanModeDiff
)

func ExampleConfig() Config {
	return corepkg.ExampleConfig()
}

func DefaultConfigPath() string {
	return corepkg.DefaultConfigPath()
}

func LoadConfigFile(path string) (Config, error) {
	return corepkg.LoadConfigFile(path)
}

func WriteConfigFile(path string, cfg Config) error {
	return corepkg.WriteConfigFile(path, cfg)
}

func ValidateConfig(cfg Config) error {
	return corepkg.ValidateConfig(cfg)
}

func NewRunner(cfg Config) *Runner {
	return corepkg.NewRunner(cfg)
}

func WriteReport(w io.Writer, report Report, format string) error {
	return corepkg.WriteReport(w, report, format)
}

func Run(ctx context.Context, cfg Config) (Report, error) {
	return corepkg.Run(ctx, cfg)
}

func RunWithOptions(ctx context.Context, cfg Config, opts ScanOptions) (Report, error) {
	return corepkg.RunWithOptions(ctx, cfg, opts)
}
