package design

import (
	"encoding/json"
	"path"
	"strings"
)

type typeScriptImportResolver struct {
	graph           *moduleGraph
	configs         []typeScriptGraphConfig
	packages        map[string]typeScriptWorkspacePackage
	tsconfigs       map[string]typeScriptConfigDocument
	tsconfigPrimary map[string]string
}

type typeScriptGraphConfig struct {
	dir     string
	baseDir string
	paths   []typeScriptPathAlias
}

type typeScriptPathAlias struct {
	pattern string
	targets []string
}

type typeScriptWorkspacePackage struct {
	name    string
	dir     string
	main    string
	module  string
	source  string
	types   string
	exports map[string][]string
	imports map[string][]string
}

type typeScriptPackageManifest struct {
	Name    string          `json:"name"`
	Main    string          `json:"main"`
	Module  string          `json:"module"`
	Source  string          `json:"source"`
	Types   string          `json:"types"`
	Exports json.RawMessage `json:"exports"`
	Imports json.RawMessage `json:"imports"`
}

type typeScriptConfigDocument struct {
	Extends         string                    `json:"extends"`
	CompilerOptions typeScriptCompilerOptions `json:"compilerOptions"`
}

type typeScriptCompilerOptions struct {
	BaseURL string              `json:"baseUrl"`
	Paths   map[string][]string `json:"paths"`
}

func newTypeScriptImportResolver(graph *moduleGraph) *typeScriptImportResolver {
	return &typeScriptImportResolver{
		graph:           graph,
		packages:        make(map[string]typeScriptWorkspacePackage),
		tsconfigs:       make(map[string]typeScriptConfigDocument),
		tsconfigPrimary: make(map[string]string),
	}
}

func isTypeScriptResolverMetadataFile(rel string) bool {
	base := path.Base(rel)
	if base == "package.json" {
		return true
	}
	return strings.HasSuffix(base, ".json")
}
