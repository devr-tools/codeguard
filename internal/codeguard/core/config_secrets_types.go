package core

// SecretsRulesConfig tunes the hardcoded secret/credential scan. The scan runs
// repository-wide across every target language and reports in both full and
// diff scans. It is enabled by default.
type SecretsRulesConfig struct {
	// Enabled toggles the whole secret scan. Defaults to true when unset.
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	// AllowPaths are glob patterns (e.g. "testdata/**") whose files are skipped.
	AllowPaths []string `json:"allow_paths,omitempty" yaml:"allow_paths,omitempty"`
	// AllowPatterns are regexes; a line matching any of them is never reported.
	AllowPatterns []string `json:"allow_patterns,omitempty" yaml:"allow_patterns,omitempty"`
	// CustomPatterns add repo-specific high-confidence credential formats.
	CustomPatterns []CustomSecretPattern `json:"custom_patterns,omitempty" yaml:"custom_patterns,omitempty"`
	// Entropy enables the opt-in high-entropy string heuristic.
	Entropy *SecretsEntropyConfig `json:"entropy,omitempty" yaml:"entropy,omitempty"`
}

// SecretsEntropyConfig tunes the optional Shannon-entropy heuristic that catches
// unknown/random secrets matching no known format. It is disabled by default.
type SecretsEntropyConfig struct {
	// Enabled toggles the entropy pass. Defaults to false when unset.
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	// MinLength is the minimum literal length considered. Defaults to 20.
	MinLength int `json:"min_length,omitempty" yaml:"min_length,omitempty"`
	// Threshold is the minimum Shannon entropy in bits/char to report. Defaults to 4.5.
	Threshold float64 `json:"threshold,omitempty" yaml:"threshold,omitempty"`
	// Level is the finding level (warn or fail). Defaults to warn.
	Level string `json:"level,omitempty" yaml:"level,omitempty"`
}

// CustomSecretPattern is a config-defined credential format. A match reports
// under the supplied id at the supplied level (defaults to "fail").
type CustomSecretPattern struct {
	ID      string `json:"id" yaml:"id"`
	Regex   string `json:"regex" yaml:"regex"`
	Message string `json:"message,omitempty" yaml:"message,omitempty"`
	Level   string `json:"level,omitempty" yaml:"level,omitempty"`
}
