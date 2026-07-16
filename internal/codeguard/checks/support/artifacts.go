package support

import "github.com/devr-tools/codeguard/internal/codeguard/core"

const ArtifactKindDependencyGraph = "dependency_graph"
const ArtifactKindSlopScore = "slop_score"
const ArtifactKindPerformanceScore = "performance_score"
const ArtifactKindChangeRisk = "change_risk"
const ArtifactKindRepoLegibility = "repo_legibility"

func NewDependencyGraphArtifact(id string, language string, target string, graph DependencyGraph) core.Artifact {
	nodes := make([]core.DependencyGraphNode, 0, len(graph.Order))
	for _, nodeID := range graph.Order {
		node := graph.Nodes[nodeID]
		edges := make([]core.DependencyGraphEdge, 0, len(node.Edges))
		for _, edge := range node.Edges {
			edges = append(edges, core.DependencyGraphEdge{
				To:    edge.To,
				Line:  edge.Line,
				Names: append([]string(nil), edge.Names...),
			})
		}
		nodes = append(nodes, core.DependencyGraphNode{
			ID:       node.ID,
			Path:     node.Path,
			IsPublic: node.IsPublic,
			Edges:    edges,
		})
	}
	return core.Artifact{
		ID:       id,
		Kind:     ArtifactKindDependencyGraph,
		Language: language,
		Target:   target,
		DependencyGraph: &core.DependencyGraphArtifact{
			Order: append([]string(nil), graph.Order...),
			Nodes: nodes,
		},
	}
}

func NewSlopScoreArtifact(id string, language string, target string, score core.SlopScoreArtifact) core.Artifact {
	components := make([]core.SlopScoreComponent, 0, len(score.Components))
	components = append(components, score.Components...)
	return core.Artifact{
		ID:       id,
		Kind:     ArtifactKindSlopScore,
		Language: language,
		Target:   target,
		SlopScore: &core.SlopScoreArtifact{
			Score:      score.Score,
			Signals:    score.Signals,
			Components: components,
		},
	}
}

func NewPerformanceScoreArtifact(id string, language string, target string, score core.PerformanceScoreArtifact) core.Artifact {
	components := make([]core.SlopScoreComponent, 0, len(score.Components))
	components = append(components, score.Components...)
	return core.Artifact{
		ID:       id,
		Kind:     ArtifactKindPerformanceScore,
		Language: language,
		Target:   target,
		PerformanceScore: &core.PerformanceScoreArtifact{
			Score:      score.Score,
			Signals:    score.Signals,
			Components: components,
		},
	}
}

func NewRepoLegibilityArtifact(id string, target string, legibility core.RepoLegibilityArtifact) core.Artifact {
	components := make([]core.RepoLegibilityComponent, 0, len(legibility.Components))
	components = append(components, legibility.Components...)
	return core.Artifact{
		ID:     id,
		Kind:   ArtifactKindRepoLegibility,
		Target: target,
		RepoLegibility: &core.RepoLegibilityArtifact{
			Score:      legibility.Score,
			Components: components,
		},
	}
}

func NewChangeRiskArtifact(id string, language string, target string, risk core.ChangeRiskArtifact) core.Artifact {
	components := make([]core.ChangeRiskComponent, 0, len(risk.Components))
	components = append(components, risk.Components...)
	return core.Artifact{
		ID:       id,
		Kind:     ArtifactKindChangeRisk,
		Language: language,
		Target:   target,
		ChangeRisk: &core.ChangeRiskArtifact{
			Score:                risk.Score,
			Level:                risk.Level,
			ProvenanceActive:     risk.ProvenanceActive,
			ChangedFiles:         risk.ChangedFiles,
			HighImpactChange:     risk.HighImpactChange,
			AIFindingCount:       risk.AIFindingCount,
			SemanticFindingCount: risk.SemanticFindingCount,
			Components:           components,
		},
	}
}
