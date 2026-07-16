package support

import (
	"path/filepath"
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

func ScanCPPFiles(env Context, target core.TargetConfig, section string, scan func(file string, data []byte) []core.Finding) []core.Finding {
	return env.ScanTargetFiles(target, section, func(rel string) bool {
		switch strings.ToLower(filepath.Ext(rel)) {
		case ".cc", ".cp", ".cpp", ".cxx", ".c++", ".hh", ".hpp", ".hxx", ".h++", ".ipp", ".tpp":
			return true
		default:
			return false
		}
	}, scan)
}
