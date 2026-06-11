package codeguard

import "github.com/devr-tools/codeguard/codeguard/config"

func ExampleConfig() Config {
	return config.ExampleConfig()
}

func DefaultConfigPath() string {
	return config.DefaultConfigPath()
}

func LoadConfigFile(path string) (Config, error) {
	return config.LoadConfigFile(path)
}

func WriteConfigFile(path string, cfg Config) error {
	return config.WriteConfigFile(path, cfg)
}

func ValidateConfig(cfg Config) error {
	return config.Validate(cfg)
}
