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
		// A target that explicitly declares C++ resolves the otherwise ambiguous
		// .h and inline/template header suffixes as C++. Standalone grammar
		// discovery remains conservative for .h; see ScriptLanguageForPath.
		return IsCPPPath(rel, true)
	}, scan)
}

// IsCPPPath reports whether path uses a conventional C++ source, header, or
// C++20 module-interface suffix. includeAmbiguousHeaders is reserved for an
// explicitly C++ target, where .h/.inc cannot be mistaken for C or another
// language.
func IsCPPPath(path string, includeAmbiguousHeaders bool) bool {
	rawExt := filepath.Ext(path)
	if rawExt == ".C" { // conventional case-sensitive Unix C++ source suffix
		return true
	}
	ext := strings.ToLower(rawExt)
	switch ext {
	case ".cc", ".cp", ".cpp", ".cxx", ".c++",
		".hh", ".hpp", ".hxx", ".h++", ".ipp", ".tpp", ".inl", ".txx",
		".ixx", ".cppm", ".cxxm", ".ccm", ".c++m", ".mpp", ".mxx", ".ii":
		return true
	case ".h", ".inc":
		return includeAmbiguousHeaders
	default:
		return false
	}
}
