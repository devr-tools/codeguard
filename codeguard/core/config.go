package core

type Config struct {
	Name    string         `json:"name"`
	Targets []TargetConfig `json:"targets"`
	Checks  CheckConfig    `json:"checks"`
	Output  OutputConfig   `json:"output"`
}

type TargetConfig struct {
	Name        string   `json:"name"`
	Path        string   `json:"path"`
	Language    string   `json:"language"`
	Entrypoints []string `json:"entrypoints,omitempty"`
}

type CheckConfig struct {
	Quality       bool                `json:"quality"`
	Design        bool                `json:"design"`
	Security      bool                `json:"security"`
	Prompts       bool                `json:"prompts"`
	CI            bool                `json:"ci"`
	QualityRules  QualityRulesConfig  `json:"quality_rules,omitempty"`
	DesignRules   DesignRulesConfig   `json:"design_rules,omitempty"`
	PromptRules   PromptRulesConfig   `json:"prompt_rules,omitempty"`
	CIRules       CIRulesConfig       `json:"ci_rules,omitempty"`
	SecurityRules SecurityRulesConfig `json:"security_rules,omitempty"`
}

type QualityRulesConfig struct {
	MaxFileLines            int `json:"max_file_lines,omitempty"`
	MaxFunctionLines        int `json:"max_function_lines,omitempty"`
	MaxParameters           int `json:"max_parameters,omitempty"`
	MaxCyclomaticComplexity int `json:"max_cyclomatic_complexity,omitempty"`
}

type SecurityRulesConfig struct {
	GovulncheckMode    string `json:"govulncheck_mode,omitempty"`
	GovulncheckCommand string `json:"govulncheck_command,omitempty"`
}

type DesignRulesConfig struct {
	RequireCmdThroughInternalCLI *bool    `json:"require_cmd_through_internal_cli,omitempty"`
	ForbidInternalImportCmd      *bool    `json:"forbid_internal_import_cmd,omitempty"`
	ForbidServiceImportInternal  *bool    `json:"forbid_service_import_internal,omitempty"`
	ForbidServiceImportCmd       *bool    `json:"forbid_service_import_cmd,omitempty"`
	MaxDeclsPerFile              int      `json:"max_decls_per_file,omitempty"`
	MaxMethodsPerType            int      `json:"max_methods_per_type,omitempty"`
	MaxInterfaceMethods          int      `json:"max_interface_methods,omitempty"`
	ForbiddenPackageNames        []string `json:"forbidden_package_names,omitempty"`
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
}

type WorkflowRuleConfig struct {
	Path             string   `json:"path"`
	RequiredContains []string `json:"required_contains,omitempty"`
}

type OutputConfig struct {
	Format string `json:"format"`
}
