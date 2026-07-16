package support

import (
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func ScanGoFiles(env Context, target core.TargetConfig, section string, scan func(file string, data []byte) []core.Finding) []core.Finding {
	return env.ScanTargetFiles(target, section, func(rel string) bool {
		return strings.HasSuffix(rel, ".go")
	}, scan)
}

func ScanPythonFiles(env Context, target core.TargetConfig, section string, scan func(file string, data []byte) []core.Finding) []core.Finding {
	return env.ScanTargetFiles(target, section, func(rel string) bool {
		return strings.HasSuffix(strings.ToLower(rel), ".py")
	}, scan)
}

func ScanRustFiles(env Context, target core.TargetConfig, section string, scan func(file string, data []byte) []core.Finding) []core.Finding {
	return env.ScanTargetFiles(target, section, func(rel string) bool {
		return strings.HasSuffix(strings.ToLower(rel), ".rs")
	}, scan)
}
