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
	SourceFiles             []string             `json:"source_files"`
	ForbiddenPackageNames   []string             `json:"forbidden_package_names"`
	MaxMethodsPerType       int                  `json:"max_methods_per_type"`
	MaxInterfaceMembers     int                  `json:"max_interface_members"`
	MaxFunctionLines        int                  `json:"max_function_lines"`
	MaxParameters           int                  `json:"max_parameters"`
	MaxCyclomaticComplexity int                  `json:"max_cyclomatic_complexity"`
	TaintModel              TypeScriptTaintModel `json:"taint_model"`
	TaintMaxDepth           int                  `json:"taint_max_depth"`
}

func newTypeScriptSemanticInput(target core.TargetConfig, cfg core.Config, libPath string, sourceFiles []string) typeScriptSemanticInput {
	return typeScriptSemanticInput{
		TypeScriptLibPath:       libPath,
		TargetPath:              target.Path,
		SourceFiles:             semanticSourcePaths(target.Path, sourceFiles),
		ForbiddenPackageNames:   append([]string(nil), cfg.Checks.DesignRules.ForbiddenPackageNames...),
		MaxMethodsPerType:       cfg.Checks.DesignRules.MaxMethodsPerType,
		MaxInterfaceMembers:     cfg.Checks.DesignRules.MaxInterfaceMethods,
		MaxFunctionLines:        cfg.Checks.QualityRules.MaxFunctionLines,
		MaxParameters:           cfg.Checks.QualityRules.MaxParameters,
		MaxCyclomaticComplexity: cfg.Checks.QualityRules.MaxCyclomaticComplexity,
		TaintModel:              defaultTypeScriptTaintModel(),
		TaintMaxDepth:           cfg.Checks.SecurityRules.TypeScriptTaintMaxDepth,
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
