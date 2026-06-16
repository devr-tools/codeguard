package config

import "github.com/devr-tools/codeguard/internal/codeguard/core"

const defaultMinChangedLineCoverage = 60

func applyCoverageDeltaDefaults(dst *core.CoverageDeltaConfig) {
	if dst.Enabled == nil {
		dst.Enabled = boolPtr(false)
	}
	if dst.MinChangedLineCoverage == nil {
		dst.MinChangedLineCoverage = intPtr(defaultMinChangedLineCoverage)
	}
}

func applyTestQualityDefaults(dst *core.TestQualityRulesConfig) {
	if dst.Enabled == nil {
		dst.Enabled = boolPtr(true)
	}
}

func intPtr(v int) *int {
	return &v
}
