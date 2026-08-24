package support

import (
	"path/filepath"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type TypeScriptSemanticResults struct {
	Design   []FindingInput `json:"design"`
	Quality  []FindingInput `json:"quality"`
	Security []FindingInput `json:"security"`
	Debug    []string       `json:"debug,omitempty"`
}

type typeScriptSemanticInput struct {
	TypeScriptLibPath       string               `json:"typescript_lib_path"`
	TargetPath              string               `json:"target_path"`
	TargetLanguage          string               `json:"target_language"`
	SourceFiles             []string             `json:"source_files"`
	ForbiddenPackageNames   []string             `json:"forbidden_package_names"`
	MaxMethodsPerType       int                  `json:"max_methods_per_type"`
	MaxInterfaceMembers     int                  `json:"max_interface_members"`
	MaxFunctionLines        int                  `json:"max_function_lines"`
	MaxParameters           int                  `json:"max_parameters"`
	MaxCyclomaticComplexity int                  `json:"max_cyclomatic_complexity"`
	DeadCode                typeScriptDeadCode   `json:"dead_code"`
	TaintModel              TypeScriptTaintModel `json:"taint_model"`
	TaintMaxDepth           int                  `json:"taint_max_depth"`
}

type typeScriptDeadCode struct {
	Enabled               bool     `json:"enabled"`
	Level                 string   `json:"level"`
	IncludeTests          bool     `json:"include_tests"`
	TypeScriptProjects    []string `json:"typescript_projects"`
	TypeScriptEntrypoints []string `json:"typescript_entrypoints"`
	TypeScriptReports     []string `json:"typescript_reports"`
	TypeScriptIgnorePaths []string `json:"typescript_ignore_paths"`
	JavaScriptProjects    []string `json:"javascript_projects"`
	JavaScriptEntrypoints []string `json:"javascript_entrypoints"`
	JavaScriptReports     []string `json:"javascript_reports"`
	JavaScriptIgnorePaths []string `json:"javascript_ignore_paths"`
}

func newTypeScriptSemanticInput(target core.TargetConfig, cfg core.Config, libPath string, sourceFiles []string) typeScriptSemanticInput {
	return typeScriptSemanticInput{
		TypeScriptLibPath:       libPath,
		TargetPath:              target.Path,
		TargetLanguage:          NormalizedLanguage(target.Language),
		SourceFiles:             semanticSourcePaths(target.Path, sourceFiles),
		ForbiddenPackageNames:   append([]string(nil), cfg.Checks.DesignRules.ForbiddenPackageNames...),
		MaxMethodsPerType:       cfg.Checks.DesignRules.MaxMethodsPerType,
		MaxInterfaceMembers:     cfg.Checks.DesignRules.MaxInterfaceMethods,
		MaxFunctionLines:        cfg.Checks.QualityRules.MaxFunctionLines,
		MaxParameters:           cfg.Checks.QualityRules.MaxParameters,
		MaxCyclomaticComplexity: cfg.Checks.QualityRules.MaxCyclomaticComplexity,
		DeadCode:                newTypeScriptDeadCodeInput(cfg.Checks.QualityRules.DeadCode),
		TaintModel:              defaultTypeScriptTaintModel(),
		TaintMaxDepth:           cfg.Checks.SecurityRules.TypeScriptTaintMaxDepth,
	}
}

func newTypeScriptDeadCodeInput(cfg core.QualityDeadCodeConfig) typeScriptDeadCode {
	enabled := false
	if cfg.Enabled != nil && *cfg.Enabled {
		switch NormalizedLanguage(cfg.Mode) {
		case "", "toolchain":
			enabled = true
		}
	}
	level := "warn"
	if NormalizedLanguage(cfg.Level) == "fail" {
		level = "fail"
	}
	includeTests := false
	if cfg.IncludeTests != nil {
		includeTests = *cfg.IncludeTests
	}
	return typeScriptDeadCode{
		Enabled:               enabled,
		Level:                 level,
		IncludeTests:          includeTests,
		TypeScriptProjects:    append([]string(nil), cfg.TypeScript.Projects...),
		TypeScriptEntrypoints: append([]string(nil), cfg.TypeScript.Entrypoints...),
		TypeScriptReports:     append([]string(nil), cfg.TypeScript.Reports...),
		TypeScriptIgnorePaths: append([]string(nil), cfg.TypeScript.IgnorePaths...),
		JavaScriptProjects:    append([]string(nil), cfg.JavaScript.Projects...),
		JavaScriptEntrypoints: append([]string(nil), cfg.JavaScript.Entrypoints...),
		JavaScriptReports:     append([]string(nil), cfg.JavaScript.Reports...),
		JavaScriptIgnorePaths: append([]string(nil), cfg.JavaScript.IgnorePaths...),
	}
}

func semanticSourcePaths(root string, sourceFiles []string) []string {
	if sourceFiles == nil {
		return nil
	}
	paths := make([]string, 0, len(sourceFiles))
	for _, file := range sourceFiles {
		paths = append(paths, filepath.Join(root, file))
	}
	return paths
}
