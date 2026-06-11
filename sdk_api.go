package codeguard

import (
	"context"
	"io"

	corepkg "github.com/devr-tools/codeguard/codeguard"
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
