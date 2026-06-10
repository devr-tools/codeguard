package codeguard

import (
	"context"
	"io"

	corepkg "github.com/devr-tools/codeguard/codeguard"
)

type Config = corepkg.Config
type TargetConfig = corepkg.TargetConfig
type CheckConfig = corepkg.CheckConfig
type OutputConfig = corepkg.OutputConfig
type Report = corepkg.Report
type SectionResult = corepkg.SectionResult
type Finding = corepkg.Finding
type Runner = corepkg.Runner

func ExampleConfig() Config {
	return corepkg.ExampleConfig()
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
