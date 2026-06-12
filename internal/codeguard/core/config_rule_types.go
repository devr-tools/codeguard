package core

type QualityRulesConfig struct {
	MaxFileLines            int                             `json:"max_file_lines"`
	MaxFunctionLines        int                             `json:"max_function_lines"`
	MaxParameters           int                             `json:"max_parameters"`
	MaxCyclomaticComplexity int                             `json:"max_cyclomatic_complexity"`
	CloneTokenThreshold     int                             `json:"clone_token_threshold,omitempty"`
	LanguageCommands        map[string][]CommandCheckConfig `json:"language_commands,omitempty"`
	AIProvenance            AIProvenanceConfig              `json:"ai_provenance,omitempty"`
	AIChecks                AIChecksConfig                  `json:"ai_checks,omitempty"`
}

// AIChecksConfig toggles individual AI-quality heuristics. A nil pointer
// leaves the check enabled, matching the rest of the rule pack defaults.
type AIChecksConfig struct {
	HallucinatedImport *bool `json:"hallucinated_import,omitempty"`
	DeadCode           *bool `json:"dead_code,omitempty"`
	ErrorStyleDrift    *bool `json:"error_style_drift,omitempty"`
	NamingDrift        *bool `json:"naming_drift,omitempty"`
	SlopHistory        *bool `json:"slop_history,omitempty"`
	SlopHistoryLimit   int   `json:"slop_history_limit,omitempty"`
}

type AIProvenanceConfig struct {
	Enabled                *bool    `json:"enabled,omitempty"`
	EnvVars                []string `json:"env_vars,omitempty"`
	CommitTrailers         []string `json:"commit_trailers,omitempty"`
	SlopScoreWarnThreshold int      `json:"slop_score_warn_threshold,omitempty"`
	SlopScoreFailThreshold int      `json:"slop_score_fail_threshold,omitempty"`
}

type DesignRulesConfig struct {
	RequireCmdThroughInternalCLI *bool                           `json:"require_cmd_through_internal_cli,omitempty"`
	ForbidInternalImportCmd      *bool                           `json:"forbid_internal_import_cmd,omitempty"`
	ForbidServiceImportInternal  *bool                           `json:"forbid_service_import_internal,omitempty"`
	ForbidServiceImportCmd       *bool                           `json:"forbid_service_import_cmd,omitempty"`
	MaxDeclsPerFile              int                             `json:"max_decls_per_file"`
	MaxMethodsPerType            int                             `json:"max_methods_per_type"`
	MaxInterfaceMethods          int                             `json:"max_interface_methods"`
	ForbiddenPackageNames        []string                        `json:"forbidden_package_names,omitempty"`
	LanguageCommands             map[string][]CommandCheckConfig `json:"language_commands,omitempty"`
	LanguageDiffCommands         map[string][]CommandCheckConfig `json:"language_diff_commands,omitempty"`
}

type PromptRulesConfig struct {
	FileExtensions            []string `json:"file_extensions,omitempty"`
	PathContains              []string `json:"path_contains,omitempty"`
	ForbidSecretInterpolation *bool    `json:"forbid_secret_interpolation,omitempty"`
	ForbidUnsafeInstructions  *bool    `json:"forbid_unsafe_instructions,omitempty"`
}

type CIRulesConfig struct {
	RequireWorkflowDir      *bool                `json:"require_workflow_dir,omitempty"`
	RequiredWorkflowFiles   []string             `json:"required_workflow_files,omitempty"`
	WorkflowContentRules    []WorkflowRuleConfig `json:"workflow_content_rules,omitempty"`
	RequiredReleaseFiles    []string             `json:"required_release_files,omitempty"`
	RequiredAutomationPaths []string             `json:"required_automation_paths,omitempty"`
	AllowedTestPaths        []string             `json:"allowed_test_paths,omitempty"`
}

type WorkflowRuleConfig struct {
	Path             string   `json:"path"`
	RequiredContains []string `json:"required_contains,omitempty"`
}

type SecurityRulesConfig struct {
	GovulncheckMode    string                          `json:"govulncheck_mode,omitempty"`
	GovulncheckCommand string                          `json:"govulncheck_command,omitempty"`
	LanguageCommands   map[string][]CommandCheckConfig `json:"language_commands,omitempty"`
}

type CommandCheckConfig struct {
	Name    string   `json:"name"`
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
}
