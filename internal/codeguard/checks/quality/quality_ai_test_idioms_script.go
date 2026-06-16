package quality

import (
	"path/filepath"
	"regexp"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var scriptTestFilePattern = regexp.MustCompile(`(?i)(?:^|/).*(?:\.test|\.spec)\.(?:[cm]?[jt]sx?)$`)

func isScriptTestFile(rel string) bool {
	return scriptTestFilePattern.MatchString(filepath.ToSlash(rel))
}

func dominantScriptTestFramework(root string, files []string, manifest packageManifest) string {
	counts := frameworkSeedCounts(manifest)
	for _, rel := range files {
		framework, include := readFrameworkFile(root, rel, isScriptTestFile, scriptTestFramework)
		if !include || framework == "" {
			continue
		}
		counts[framework]++
	}
	return dominantFrameworkFromCounts(counts)
}

func scriptTestFramework(source string) string {
	switch {
	case containsAny(source, []string{`from "vitest"`, "from 'vitest'", "vi.mock(", "vi.fn("}):
		return "vitest"
	case containsAny(source, []string{`from "@jest/globals"`, "from '@jest/globals'", "jest.mock(", "jest.fn("}):
		return "jest"
	case containsAny(source, []string{`from "mocha"`, "from 'mocha'", "sinon.stub("}):
		return "mocha"
	default:
		return ""
	}
}

func scriptIdiomDriftFinding(env support.Context, file string, source string, dominant string) []core.Finding {
	return idiomDriftFinding(env, file, dominant, scriptTestFramework(source))
}

func frameworkSeedCounts(manifest packageManifest) map[string]int {
	counts := map[string]int{}
	for _, framework := range []string{"vitest", "jest", "mocha"} {
		if containsPackage(manifest, framework) {
			counts[framework] += 3
		}
	}
	return counts
}

func containsPackage(manifest packageManifest, pkg string) bool {
	_, ok := packageManifestDeps(manifest)[pkg]
	return ok
}
