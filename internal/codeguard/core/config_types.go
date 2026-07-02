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
	Parsers   ParsersConfig    `json:"parsers,omitempty" yaml:"parsers,omitempty"`
}

// ParsersConfig selects the parsing substrate for non-Go languages.
type ParsersConfig struct {
	// TreeSitter controls the tree-sitter parsing path for
	// TypeScript/TSX/JavaScript rules: "off" (default) keeps the regex-based
	// scanners exactly as they are; "auto" parses script files through the
	// embedded tree-sitter grammars and falls back to the regex path per file
	// on parse failure, oversized input, or error-heavy trees.
	TreeSitter string `json:"treesitter,omitempty" yaml:"treesitter,omitempty"`
}

// Tree-sitter parser modes accepted by ParsersConfig.TreeSitter.
const (
	TreeSitterModeOff  = "off"
	TreeSitterModeAuto = "auto"
)

// TreeSitterEnabled reports whether the tree-sitter parsing path is enabled
// (mode "auto"). Empty and "off" both disable it.
func (p ParsersConfig) TreeSitterEnabled() bool {
	return p.TreeSitter == TreeSitterModeAuto
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
	Contracts *bool `json:"contracts,omitempty" yaml:"contracts,omitempty"`
	// Context toggles the agent-context legibility family: checks for how
	// navigable and trustworthy the repository is for AI coding agents (agent
	// instruction docs, doc/README drift, context-budget file sizes, and
	// basename ambiguity). When nil it defaults to enabled in full scans and
	// disabled in diff scans, whose repo-level findings would repeat on every
	// PR regardless of the change under review.
	Context          *bool                  `json:"context,omitempty" yaml:"context,omitempty"`
	QualityRules     QualityRulesConfig     `json:"quality_rules" yaml:"quality_rules"`
	DesignRules      DesignRulesConfig      `json:"design_rules" yaml:"design_rules"`
	PromptRules      PromptRulesConfig      `json:"prompt_rules" yaml:"prompt_rules"`
	CIRules          CIRulesConfig          `json:"ci_rules" yaml:"ci_rules"`
	SecurityRules    SecurityRulesConfig    `json:"security_rules" yaml:"security_rules"`
	SupplyChainRules SupplyChainRulesConfig `json:"supply_chain_rules" yaml:"supply_chain_rules"`
	ContractRules    ContractRulesConfig    `json:"contract_rules" yaml:"contract_rules"`
	ContextRules     ContextRulesConfig     `json:"context_rules" yaml:"context_rules"`
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
