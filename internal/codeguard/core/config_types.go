package core

type Config struct {
	Name      string           `json:"name" yaml:"name"`
	Profile   string           `json:"profile,omitempty" yaml:"profile,omitempty"`
	Targets   []TargetConfig   `json:"targets" yaml:"targets"`
	Checks    CheckConfig      `json:"checks" yaml:"checks"`
	AI        AIConfig         `json:"ai,omitempty" yaml:"ai,omitempty"`
	RulePacks []RulePackConfig `json:"rule_packs,omitempty" yaml:"rule_packs,omitempty"`
	Output    OutputConfig     `json:"output" yaml:"output"`
	Exclude   []string         `json:"exclude,omitempty" yaml:"exclude,omitempty"`
	Baseline  BaselineConfig   `json:"baseline,omitempty" yaml:"baseline,omitempty"`
	Waivers   []WaiverConfig   `json:"waivers,omitempty" yaml:"waivers,omitempty"`
	Cache     CacheConfig      `json:"cache,omitempty" yaml:"cache,omitempty"`
}

type TargetConfig struct {
	Name        string   `json:"name" yaml:"name"`
	Path        string   `json:"path" yaml:"path"`
	Language    string   `json:"language" yaml:"language"`
	Entrypoints []string `json:"entrypoints,omitempty" yaml:"entrypoints,omitempty"`
}

type CheckConfig struct {
	Quality  bool `json:"quality" yaml:"quality"`
	Design   bool `json:"design" yaml:"design"`
	Security bool `json:"security" yaml:"security"`
	Prompts  bool `json:"prompts" yaml:"prompts"`
	CI       bool `json:"ci" yaml:"ci"`
	// SupplyChain toggles dependency-policy checks such as manifest hygiene,
	// lockfile drift, license policy, and SBOM-oriented validation.
	SupplyChain bool `json:"supply_chain,omitempty" yaml:"supply_chain,omitempty"`
	// Contracts toggles the API contract drift family. When nil it defaults
	// to enabled in diff scans and disabled in full scans; the strict and
	// enterprise profiles enable it unconditionally.
	Contracts        *bool                  `json:"contracts,omitempty" yaml:"contracts,omitempty"`
	QualityRules     QualityRulesConfig     `json:"quality_rules" yaml:"quality_rules"`
	DesignRules      DesignRulesConfig      `json:"design_rules" yaml:"design_rules"`
	PromptRules      PromptRulesConfig      `json:"prompt_rules" yaml:"prompt_rules"`
	CIRules          CIRulesConfig          `json:"ci_rules" yaml:"ci_rules"`
	SecurityRules    SecurityRulesConfig    `json:"security_rules" yaml:"security_rules"`
	SupplyChainRules SupplyChainRulesConfig `json:"supply_chain_rules" yaml:"supply_chain_rules"`
	ContractRules    ContractRulesConfig    `json:"contract_rules" yaml:"contract_rules"`
}

type OutputConfig struct {
	Format string `json:"format" yaml:"format"`
}

type CacheConfig struct {
	Enabled *bool  `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Path    string `json:"path,omitempty" yaml:"path,omitempty"`
}

type BaselineConfig struct {
	Path string `json:"path,omitempty" yaml:"path,omitempty"`
}

type WaiverConfig struct {
	Rule      string `json:"rule" yaml:"rule"`
	Path      string `json:"path,omitempty" yaml:"path,omitempty"`
	Reason    string `json:"reason,omitempty" yaml:"reason,omitempty"`
	ExpiresOn string `json:"expires_on,omitempty" yaml:"expires_on,omitempty"`
}
