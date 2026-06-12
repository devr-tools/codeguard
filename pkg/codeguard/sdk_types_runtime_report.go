package codeguard

import "github.com/devr-tools/codeguard/internal/codeguard/core"

type (
	Report             = core.Report
	Artifact           = core.Artifact
	SlopScoreArtifact  = core.SlopScoreArtifact
	SlopScoreComponent = core.SlopScoreComponent
	SlopHistoryEntry   = core.SlopHistoryEntry
	AIAnalysisArtifact = core.AIAnalysisArtifact
	AIAnalysisVerdict  = core.AIAnalysisVerdict
	AIFixArtifact      = core.AIFixArtifact
	SectionResult      = core.SectionResult
	Finding            = core.Finding

	ChangeImpactArtifact = core.ChangeImpactArtifact
	ChangeImpactEntry    = core.ChangeImpactEntry
)
