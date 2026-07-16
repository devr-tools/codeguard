package support

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var cmakeVariablePattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

func parseCMakeManifest(_ string, rel string, data []byte) core.SupplyChainManifest {
	manifest := core.SupplyChainManifest{
		Ecosystem:      "cmake",
		PackageManager: "cmake",
		Path:           rel,
	}
	variables := make(map[string]string)
	for _, command := range scanCMakeCommands(string(data)) {
		args, resolved := resolveCMakeArguments(command.args, variables)
		switch strings.ToLower(command.name) {
		case "set":
			rememberCMakeVariable(variables, args, resolved)
		case "find_package":
			if dep, ok := parseCMakeFindPackage(args, command.line); ok {
				manifest.Dependencies = append(manifest.Dependencies, dep)
			}
			appendCMakeResolutionLimitation(&manifest, command, resolved)
		case "fetchcontent_declare", "externalproject_add", "cpmaddpackage":
			if dep, ok := parseCMakeFetch(command.name, args, command.line); ok {
				manifest.Dependencies = append(manifest.Dependencies, dep)
			}
			appendCMakeResolutionLimitation(&manifest, command, resolved)
		}
	}
	sortDependencies(manifest.Dependencies)
	manifest.AnalysisLimitations = uniqueSortedStrings(manifest.AnalysisLimitations)
	return manifest
}

func appendCMakeResolutionLimitation(manifest *core.SupplyChainManifest, command cmakeCommand, resolved bool) {
	if resolved {
		return
	}
	manifest.AnalysisLimitations = append(manifest.AnalysisLimitations,
		fmt.Sprintf("%s dependency declaration at line %d contains an unresolved variable; CodeGuard did not execute CMake to resolve it", command.name, command.line))
}

func resolveCMakeArguments(args []cmakeArgument, variables map[string]string) ([]cmakeArgument, bool) {
	resolved := make([]cmakeArgument, len(args))
	allResolved := true
	for idx, arg := range args {
		value, ok := resolveCMakeValue(arg.value, variables)
		resolved[idx] = cmakeArgument{value: value, line: arg.line}
		allResolved = allResolved && ok
	}
	return resolved, allResolved
}

func resolveCMakeValue(value string, variables map[string]string) (string, bool) {
	for range 8 {
		unresolved := false
		changed := false
		value = cmakeVariablePattern.ReplaceAllStringFunc(value, func(match string) string {
			parts := cmakeVariablePattern.FindStringSubmatch(match)
			replacement, ok := variables[parts[1]]
			if !ok {
				unresolved = true
				return match
			}
			changed = true
			return replacement
		})
		if unresolved {
			return value, false
		}
		if !changed {
			return value, !strings.Contains(value, "$<") && !strings.Contains(value, "$ENV{")
		}
	}
	return value, false
}

func rememberCMakeVariable(variables map[string]string, args []cmakeArgument, resolved bool) {
	if len(args) < 2 || !resolved || !isCMakeIdentifier(args[0].value) {
		return
	}
	values := make([]string, 0, len(args)-1)
	for _, arg := range args[1:] {
		upper := strings.ToUpper(arg.value)
		if upper == "CACHE" || upper == "PARENT_SCOPE" {
			break
		}
		values = append(values, arg.value)
	}
	if len(values) > 0 {
		variables[args[0].value] = strings.Join(values, ";")
	}
}

func uniqueSortedStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	unique := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}
	slices.Sort(unique)
	return unique
}

func isCMakeIdentifier(value string) bool {
	if value == "" || !isCMakeIdentifierStart(value[0]) {
		return false
	}
	for _, char := range []byte(value[1:]) {
		if !isCMakeIdentifierPart(char) {
			return false
		}
	}
	return true
}
