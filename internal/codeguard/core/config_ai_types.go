package core

type AIConfig struct {
	Enabled      *bool                `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Provider     AIProviderConfig     `json:"provider,omitempty" yaml:"provider,omitempty"`
	Cache        AICacheConfig        `json:"cache,omitempty" yaml:"cache,omitempty"`
	HybridTriage AIHybridTriageConfig `json:"hybrid_triage,omitempty" yaml:"hybrid_triage,omitempty"`
	Semantic     AISemanticConfig     `json:"semantic,omitempty" yaml:"semantic,omitempty"`
	AutoFix      AIAutoFixConfig      `json:"autofix,omitempty" yaml:"autofix,omitempty"`
}

type AIProviderConfig struct {
	Type      string   `json:"type,omitempty" yaml:"type,omitempty"`
	Model     string   `json:"model,omitempty" yaml:"model,omitempty"`
	BaseURL   string   `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	APIKeyEnv string   `json:"api_key_env,omitempty" yaml:"api_key_env,omitempty"`
	Command   string   `json:"command,omitempty" yaml:"command,omitempty"`
	Args      []string `json:"args,omitempty" yaml:"args,omitempty"`
}

type AICacheConfig struct {
	Path string `json:"path,omitempty" yaml:"path,omitempty"`
}

type AIHybridTriageConfig struct {
	Enabled             *bool    `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	SuppressDismissed   *bool    `json:"suppress_dismissed,omitempty" yaml:"suppress_dismissed,omitempty"`
	CandidateSections   []string `json:"candidate_sections,omitempty" yaml:"candidate_sections,omitempty"`
	CandidateSeverities []string `json:"candidate_severities,omitempty" yaml:"candidate_severities,omitempty"`
}

type AISemanticConfig struct {
	Enabled                 *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	FunctionContract        *bool `json:"function_contract,omitempty" yaml:"function_contract,omitempty"`
	ContractDrift           *bool `json:"contract_drift,omitempty" yaml:"contract_drift,omitempty"`
	MisleadingErrorMessages *bool `json:"misleading_error_messages,omitempty" yaml:"misleading_error_messages,omitempty"`
	TestBehaviorCoverage    *bool `json:"test_behavior_coverage,omitempty" yaml:"test_behavior_coverage,omitempty"`
	TestAdequacy            *bool `json:"test_adequacy,omitempty" yaml:"test_adequacy,omitempty"`
}

type AIAutoFixConfig struct {
	Enabled      *bool                `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	VerifyTests  *bool                `json:"verify_tests,omitempty" yaml:"verify_tests,omitempty"`
	MaxFixes     int                  `json:"max_fixes,omitempty" yaml:"max_fixes,omitempty"`
	TestCommands []CommandCheckConfig `json:"test_commands,omitempty" yaml:"test_commands,omitempty"`
}
