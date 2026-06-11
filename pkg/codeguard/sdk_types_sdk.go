package codeguard

import (
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/internal/codeguard/runner"
)

type ReportSummary = core.ReportSummary
type RuleMetadata = core.RuleMetadata
type PolicyProfile = core.PolicyProfile
type Runner = runner.Runner

const (
	ScanModeFull = core.ScanModeFull
	ScanModeDiff = core.ScanModeDiff
	StatusPass   = core.StatusPass
	StatusWarn   = core.StatusWarn
	StatusFail   = core.StatusFail
)
