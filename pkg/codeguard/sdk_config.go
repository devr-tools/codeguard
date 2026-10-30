package codeguard

import "github.com/devr-tools/codeguard/internal/codeguard/config"

// ErrConfigNotFound identifies a missing top-level configuration file.
var ErrConfigNotFound = config.ErrConfigNotFound

// ExampleConfig returns CodeGuard's complete, ready-to-edit starter
// configuration. It is intended for callers creating a new configuration,
// rather than as a way to normalize a partial Config.
func ExampleConfig() Config {
	return config.ExampleConfig()
}

// ExampleConfigForProfile returns a complete starter configuration with the
// named built-in profile applied. It does not enable recommended defaults;
// profiles and the recommended section policy are independent.
func ExampleConfigForProfile(profile string) (Config, error) {
	return config.ExampleConfigForProfile(profile)
}

// DefaultConfigPath returns the conventional filename used when locating a
// CodeGuard configuration.
func DefaultConfigPath() string {
	return config.DefaultConfigPath()
}

// LoadConfigFile reads, defaults, and validates a configuration file.
func LoadConfigFile(path string) (Config, error) {
	return config.LoadFile(path)
}

// WriteConfigFile applies defaults, validates cfg, and writes it to path.
func WriteConfigFile(path string, cfg Config) error {
	return config.WriteFile(path, cfg)
}

// ValidateConfig checks whether cfg is a valid CodeGuard configuration without
// running a scan. Call ApplyDefaults first when validating a partial config
// constructed in memory.
func ValidateConfig(cfg Config) error {
	return config.Validate(cfg)
}

// ValidateTargetPaths checks that every target path in cfg names an existing,
// readable directory. Unlike ValidateConfig it touches the filesystem, so call
// it only where the targets are expected to exist. Relative paths resolve
// against the working directory, as they do for a scan; LoadConfig has already
// joined a loaded config's relative target paths onto the config's directory.
// Scans run this check themselves and fail rather than report a missing target
// as a clean pass.
func ValidateTargetPaths(cfg Config) error {
	return config.ValidateTargetPaths(cfg)
}

// ApplyDefaults fills omitted values on cfg in place. Use it for a partial
// Config constructed or decoded in memory; use ExampleConfig when starting a
// new configuration. When UseRecommendedDefaults is true, it additionally
// enables the recommended section baseline. Checks.Disabled is applied last.
func ApplyDefaults(cfg *Config) {
	config.ApplyDefaults(cfg)
}
