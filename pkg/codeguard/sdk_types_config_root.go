package codeguard

import "github.com/devr-tools/codeguard/internal/codeguard/core"

// Config is the complete CodeGuard configuration accepted by SDK entrypoints.
type Config = core.Config

type TargetConfig = core.TargetConfig

// CheckConfig controls which check families run and their policies. Its
// UseRecommendedDefaults and Disabled fields select the optional recommended
// section policy without changing the legacy behavior when omitted.
type CheckConfig = core.CheckConfig

type ParsersConfig = core.ParsersConfig

// ConfidencePolicyConfig is the minimum-confidence policy applied to findings,
// with one default level and optional per-section overrides.
type ConfidencePolicyConfig = core.ConfidencePolicyConfig

// Confidence levels a finding can carry, and the values accepted by
// checks.min_confidence. An empty confidence must be read as medium rather than
// as a weak finding: it means the check did not state one.
const (
	ConfidenceHigh   = core.ConfidenceHigh
	ConfidenceMedium = core.ConfidenceMedium
	ConfidenceLow    = core.ConfidenceLow
)

type ExternalReportConfig = core.ExternalReportConfig
