package codeguard

import "github.com/devr-tools/codeguard/internal/codeguard/config"

func ExampleConfig() Config {
	return config.ExampleConfig()
}

func ExampleConfigForProfile(profile string) (Config, error) {
	return config.ExampleConfigForProfile(profile)
}

func DefaultConfigPath() string {
	return config.DefaultConfigPath()
}

func LoadConfigFile(path string) (Config, error) {
	return config.LoadFile(path)
}

func WriteConfigFile(path string, cfg Config) error {
	return config.WriteFile(path, cfg)
}

func ValidateConfig(cfg Config) error {
	return config.Validate(cfg)
}

func ApplyDefaults(cfg *Config) {
	config.ApplyDefaults(cfg)
}

func Rules() []RuleMetadata {
	return config.RuleList()
}

func RulesForConfig(cfg Config) []RuleMetadata {
	return config.RuleListForConfig(cfg)
}

func ExplainRule(ruleID string) (RuleMetadata, bool) {
	return config.ExplainRule(ruleID)
}

func ExplainRuleForConfig(cfg Config, ruleID string) (RuleMetadata, bool) {
	return config.ExplainRuleForConfig(cfg, ruleID)
}
