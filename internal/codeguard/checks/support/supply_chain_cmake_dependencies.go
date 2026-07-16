package support

import (
	"path"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func parseCMakeFetch(commandName string, args []cmakeArgument, line int) (core.SupplyChainDependency, bool) {
	if len(args) == 0 {
		return core.SupplyChainDependency{}, false
	}
	values := cmakeKeywordValues(args)
	name := cmakeFetchName(commandName, args[0].value, values)
	repository := firstNonEmpty(values["GIT_REPOSITORY"], values["GITHUB_REPOSITORY"], values["GITLAB_REPOSITORY"], values["BITBUCKET_REPOSITORY"])
	url := firstNonEmpty(values["URL"], values["DOWNLOAD_URL"])
	tag, version := firstNonEmpty(values["GIT_TAG"], values["TAG"]), values["VERSION"]
	if repository == "" && url == "" && tag == "" && version == "" {
		return core.SupplyChainDependency{}, false
	}
	name = resolvedCMakeFetchName(name, repository, url)
	if name == "" {
		return core.SupplyChainDependency{}, false
	}
	requirement := cmakeFetchRequirement(repository, url, tag, version, values["URL_HASH"])
	pinned, resolvedVersion := cmakeFetchPin(url, tag, version, values["URL_HASH"])
	return core.SupplyChainDependency{
		Name:        name,
		Requirement: requirement,
		Version:     resolvedVersion,
		Scope:       "build",
		Pinned:      pinned,
		Line:        line,
	}, true
}

func cmakeFetchName(commandName string, firstArg string, values map[string]string) string {
	if !strings.EqualFold(commandName, "CPMAddPackage") {
		return firstArg
	}
	if values["NAME"] != "" {
		return values["NAME"]
	}
	name, version, ok := parseCPMCompactReference(firstArg)
	if !ok {
		return firstArg
	}
	values["VERSION"] = version
	return name
}

func resolvedCMakeFetchName(name string, repository string, url string) string {
	if name == "" || containsDynamicCMakeValue(name) {
		name = dependencyNameFromLocation(firstNonEmpty(repository, url))
	}
	if containsDynamicCMakeValue(name) {
		return ""
	}
	return name
}

func cmakeFetchRequirement(repository string, url string, tag string, version string, urlHash string) string {
	if url != "" && urlHash != "" {
		return url + "#" + urlHash
	}
	if repository != "" && tag != "" {
		return repository + "@" + tag
	}
	return firstNonEmpty(tag, version, url, repository)
}

func cmakeFetchPin(url string, tag string, version string, urlHash string) (bool, string) {
	resolvedVersion := version
	if resolvedVersion == "" && url != "" {
		resolvedVersion = versionFromCMakeURL(url)
	}
	if resolvedVersion == "" && isExactCMakeRevision(tag) {
		resolvedVersion = tag
	}
	switch {
	case urlHash != "" && !containsDynamicCMakeValue(urlHash):
		return true, resolvedVersion
	case tag != "":
		return isExactCMakeRevision(tag), resolvedVersion
	case version != "":
		return isExactCMakeVersion(version), resolvedVersion
	default:
		return resolvedVersion != "" && !containsDynamicCMakeValue(url), resolvedVersion
	}
}

func cmakeKeywordValues(args []cmakeArgument) map[string]string {
	values := make(map[string]string)
	known := map[string]struct{}{
		"NAME": {}, "VERSION": {}, "GIT_REPOSITORY": {}, "GITHUB_REPOSITORY": {},
		"GITLAB_REPOSITORY": {}, "BITBUCKET_REPOSITORY": {}, "GIT_TAG": {}, "TAG": {},
		"URL": {}, "DOWNLOAD_URL": {}, "URL_HASH": {},
	}
	for idx := 0; idx+1 < len(args); idx++ {
		key := strings.ToUpper(args[idx].value)
		if _, ok := known[key]; ok {
			values[key] = args[idx+1].value
			idx++
		}
	}
	return values
}

func parseCPMCompactReference(raw string) (string, string, bool) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "gh:") || strings.HasPrefix(raw, "gl:") || strings.HasPrefix(raw, "bb:") {
		raw = raw[3:]
	}
	name, version, ok := strings.Cut(raw, "@")
	if !ok || name == "" || version == "" {
		return "", "", false
	}
	return dependencyNameFromLocation(name), version, true
}

func dependencyNameFromLocation(location string) string {
	location = strings.TrimSuffix(strings.TrimSpace(location), "/")
	location = strings.TrimSuffix(location, ".git")
	return path.Base(location)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
