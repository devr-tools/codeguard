package codeguard

import "github.com/devr-tools/codeguard/internal/codeguard/core"

type (
	Report                      = core.Report
	Artifact                    = core.Artifact
	SupplyChainArtifact         = core.SupplyChainArtifact
	SupplyChainManifest         = core.SupplyChainManifest
	SupplyChainDependency       = core.SupplyChainDependency
	SupplyChainLicenseCandidate = core.SupplyChainLicenseCandidate
	SlopScoreArtifact           = core.SlopScoreArtifact
	SlopScoreComponent          = core.SlopScoreComponent
	ChangeRiskArtifact          = core.ChangeRiskArtifact
	ChangeRiskComponent         = core.ChangeRiskComponent
	FileRiskArtifact            = core.FileRiskArtifact
	FileRiskEntry               = core.FileRiskEntry
	FileRiskComponent           = core.FileRiskComponent
	PRHotspotsArtifact          = core.PRHotspotsArtifact
	SlopHistoryEntry            = core.SlopHistoryEntry
	PerformanceScoreArtifact    = core.PerformanceScoreArtifact
	PerformanceHistoryEntry     = core.PerformanceHistoryEntry
	RuleStatsArtifact           = core.RuleStatsArtifact
	RuleStatsEntry              = core.RuleStatsEntry
	RuleStatsHistoryEntry       = core.RuleStatsHistoryEntry
	AIAnalysisArtifact          = core.AIAnalysisArtifact
	AIAnalysisVerdict           = core.AIAnalysisVerdict
	AIFixArtifact               = core.AIFixArtifact
	SectionResult               = core.SectionResult
	Finding                     = core.Finding

	ChangeImpactArtifact = core.ChangeImpactArtifact
	ChangeImpactEntry    = core.ChangeImpactEntry

	RepoLegibilityArtifact  = core.RepoLegibilityArtifact
	RepoLegibilityComponent = core.RepoLegibilityComponent
)
