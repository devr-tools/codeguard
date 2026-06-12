package core

type Config struct {
	Name      string           `json:"name"`
	Profile   string           `json:"profile,omitempty"`
	Targets   []TargetConfig   `json:"targets"`
	Checks    CheckConfig      `json:"checks"`
	AI        AIConfig         `json:"ai,omitempty"`
	RulePacks []RulePackConfig `json:"rule_packs,omitempty"`
	Output    OutputConfig     `json:"output"`
	Exclude   []string         `json:"exclude,omitempty"`
	Baseline  BaselineConfig   `json:"baseline,omitempty"`
	Waivers   []WaiverConfig   `json:"waivers,omitempty"`
	Cache     CacheConfig      `json:"cache,omitempty"`
}

type TargetConfig struct {
	Name        string   `json:"name"`
	Path        string   `json:"path"`
	Language    string   `json:"language"`
	Entrypoints []string `json:"entrypoints,omitempty"`
}

type CheckConfig struct {
	Quality  bool `json:"quality"`
	Design   bool `json:"design"`
	Security bool `json:"security"`
	Prompts  bool `json:"prompts"`
	CI       bool `json:"ci"`
	// Contracts toggles the API contract drift family. When nil it defaults
	// to enabled in diff scans and disabled in full scans; the strict and
	// enterprise profiles enable it unconditionally.
	Contracts     *bool               `json:"contracts,omitempty"`
	QualityRules  QualityRulesConfig  `json:"quality_rules"`
	DesignRules   DesignRulesConfig   `json:"design_rules"`
	PromptRules   PromptRulesConfig   `json:"prompt_rules"`
	CIRules       CIRulesConfig       `json:"ci_rules"`
	SecurityRules SecurityRulesConfig `json:"security_rules"`
	ContractRules ContractRulesConfig `json:"contract_rules"`
}

type OutputConfig struct {
	Format string `json:"format"`
}

type CacheConfig struct {
	Enabled *bool  `json:"enabled,omitempty"`
	Path    string `json:"path,omitempty"`
}

type BaselineConfig struct {
	Path string `json:"path,omitempty"`
}

type WaiverConfig struct {
	Rule      string `json:"rule"`
	Path      string `json:"path,omitempty"`
	Reason    string `json:"reason,omitempty"`
	ExpiresOn string `json:"expires_on,omitempty"`
}
