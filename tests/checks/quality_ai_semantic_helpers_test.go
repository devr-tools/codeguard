package checks_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func qualityAISemanticConfig(dir string, name string) codeguard.Config {
	cfg := qualityAITestConfig(dir, name)
	enabled := true
	cfg.Cache.Enabled = &enabled
	cfg.Cache.Path = filepath.Join(dir, ".codeguard", "cache.json")
	return cfg
}

func semanticScript(counterPath string, response string) string {
	return "#!/bin/sh\n" +
		"count=0\n" +
		"if [ -f \"" + counterPath + "\" ]; then count=$(cat \"" + counterPath + "\"); fi\n" +
		"count=$((count + 1))\n" +
		"printf \"%s\" \"$count\" > \"" + counterPath + "\"\n" +
		"cat >/dev/null\n" +
		"printf '%s' '" + response + "'\n"
}

func semanticCaptureScript(counterPath string, requestPath string, response string) string {
	return "#!/bin/sh\n" +
		"count=0\n" +
		"if [ -f \"" + counterPath + "\" ]; then count=$(cat \"" + counterPath + "\"); fi\n" +
		"count=$((count + 1))\n" +
		"printf \"%s\" \"$count\" > \"" + counterPath + "\"\n" +
		"cat >\"" + requestPath + "\"\n" +
		"printf '%s' '" + response + "'\n"
}

func runSemanticGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, string(output))
	}
}

//nolint:unparam // general-purpose test helper; want is part of its API shape
func assertFileEquals(t *testing.T, path string, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(data) != want {
		t.Fatalf("%s = %q, want %q", path, string(data), want)
	}
}

func stringsJoin(lines ...string) string { return strings.Join(lines, "\n") }
