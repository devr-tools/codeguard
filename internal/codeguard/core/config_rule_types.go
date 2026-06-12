package core

type QualityRulesConfig struct {
	MaxFileLines               int                             `json:"max_file_lines"`
	MaxFunctionLines           int                             `json:"max_function_lines"`
	MaxParameters              int                             `json:"max_parameters"`
	MaxCyclomaticComplexity    int                             `json:"max_cyclomatic_complexity"`
	DetectNPlusOneQuery        *bool                           `json:"detect_n_plus_one_query,omitempty"`
	DetectAllocInLoop          *bool                           `json:"detect_alloc_in_loop,omitempty"`
	DetectSyncIOInHandlers     *bool                           `json:"detect_sync_io_in_handlers,omitempty"`
	DetectUnboundedConcurrency *bool                           `json:"detect_unbounded_concurrency,omitempty"`
	LanguageCommands           map[string][]CommandCheckConfig `json:"language_commands,omitempty"`
}

type DesignRulesConfig struct {
	RequireCmdThroughInternalCLI *bool                           `json:"require_cmd_through_internal_cli,omitempty"`
	ForbidInternalImportCmd      *bool                           `json:"forbid_internal_import_cmd,omitempty"`
	ForbidServiceImportInternal  *bool                           `json:"forbid_service_import_internal,omitempty"`
	ForbidServiceImportCmd       *bool                           `json:"forbid_service_import_cmd,omitempty"`
	MaxDeclsPerFile              int                             `json:"max_decls_per_file"`
	MaxMethodsPerType            int                             `json:"max_methods_per_type"`
	MaxInterfaceMethods          int                             `json:"max_interface_methods"`
	DetectImportCycles           *bool                           `json:"detect_import_cycles,omitempty"`
	DetectGodModules             *bool                           `json:"detect_god_modules,omitempty"`
	GodModuleThreshold           int                             `json:"god_module_threshold"`
	DetectHighImpactChanges      *bool                           `json:"detect_high_impact_changes,omitempty"`
	HighImpactChangeThreshold    int                             `json:"high_impact_change_threshold"`
	ForbiddenPackageNames        []string                        `json:"forbidden_package_names,omitempty"`
	LanguageCommands             map[string][]CommandCheckConfig `json:"language_commands,omitempty"`
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
