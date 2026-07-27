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
type ExternalReportConfig = core.ExternalReportConfig
