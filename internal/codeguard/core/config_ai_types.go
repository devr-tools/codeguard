package core

type AIConfig struct {
	Enabled      *bool                `json:"enabled,omitempty"`
	Provider     AIProviderConfig     `json:"provider,omitempty"`
	Cache        AICacheConfig        `json:"cache,omitempty"`
	HybridTriage AIHybridTriageConfig `json:"hybrid_triage,omitempty"`
	Semantic     AISemanticConfig     `json:"semantic,omitempty"`
	AutoFix      AIAutoFixConfig      `json:"autofix,omitempty"`
}

type AIProviderConfig struct {
	Type      string   `json:"type,omitempty"`
	Model     string   `json:"model,omitempty"`
	BaseURL   string   `json:"base_url,omitempty"`
	APIKeyEnv string   `json:"api_key_env,omitempty"`
	Command   string   `json:"command,omitempty"`
	Args      []string `json:"args,omitempty"`
}

type AICacheConfig struct {
	Path string `json:"path,omitempty"`
}

type AIHybridTriageConfig struct {
	Enabled             *bool    `json:"enabled,omitempty"`
	SuppressDismissed   *bool    `json:"suppress_dismissed,omitempty"`
	CandidateSections   []string `json:"candidate_sections,omitempty"`
	CandidateSeverities []string `json:"candidate_severities,omitempty"`
}

type AISemanticConfig struct {
	Enabled                 *bool `json:"enabled,omitempty"`
	FunctionContract        *bool `json:"function_contract,omitempty"`
	ContractDrift           *bool `json:"contract_drift,omitempty"`
	MisleadingErrorMessages *bool `json:"misleading_error_messages,omitempty"`
	TestBehaviorCoverage    *bool `json:"test_behavior_coverage,omitempty"`
	TestAdequacy            *bool `json:"test_adequacy,omitempty"`
}

type AIAutoFixConfig struct {
	Enabled      *bool                `json:"enabled,omitempty"`
	VerifyTests  *bool                `json:"verify_tests,omitempty"`
	MaxFixes     int                  `json:"max_fixes,omitempty"`
	TestCommands []CommandCheckConfig `json:"test_commands,omitempty"`
}
