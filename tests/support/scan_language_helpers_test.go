package support_test

import (
	"slices"
	"testing"

	checksupport "github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func TestScanCPPFilesIncludesExplicitTargetHeadersAndModules(t *testing.T) {
	candidates := []string{
		"include/widget.h",
		"include/widget.inc",
		"include/widget.inl",
		"include/widget.tpp",
		"src/widget.cpp",
		"src/widget.ixx",
		"src/widget.cppm",
		"src/widget.cxxm",
		"src/widget.ccm",
		"src/widget.c++m",
		"src/widget.mpp",
		"src/widget.mxx",
		"src/widget.ii",
		"src/widget.C",
		"src/widget.c",
		"README.md",
	}
	included := make([]string, 0)
	env := checksupport.Context{
		ScanTargetFiles: func(_ core.TargetConfig, _ string, include func(string) bool, _ func(string, []byte) []core.Finding) []core.Finding {
			for _, candidate := range candidates {
				if include(candidate) {
					included = append(included, candidate)
				}
			}
			return nil
		},
	}

	checksupport.ScanCPPFiles(env, core.TargetConfig{Language: "c++"}, "quality", func(string, []byte) []core.Finding { return nil })

	want := candidates[:len(candidates)-2]
	if !slices.Equal(included, want) {
		t.Fatalf("included C++ paths = %v, want %v", included, want)
	}
}

func TestCPPPathRequiresExplicitTargetForAmbiguousHeaders(t *testing.T) {
	for _, path := range []string{"widget.h", "widget.inc"} {
		if checksupport.IsCPPPath(path, false) {
			t.Errorf("IsCPPPath(%q, false) = true, want false", path)
		}
		if !checksupport.IsCPPPath(path, true) {
			t.Errorf("IsCPPPath(%q, true) = false, want true", path)
		}
	}
}
