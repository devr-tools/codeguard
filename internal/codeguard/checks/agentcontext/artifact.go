package agentcontext

import (
	"fmt"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// targetAssessment carries every measurement the legibility score aggregates
// for one target, independent of which rules are toggled on.
type targetAssessment struct {
	agentDocs       []string
	readmePresent   bool
	driftReferences int
	inventory       sourceInventory
	ambiguousGroups [][]string
	maxFileLines    int
}

// legibilityArtifact converts a target assessment into the repo_legibility
// artifact: a 0-100 score (higher is more legible to agents) with an
// explainable component breakdown.
func legibilityArtifact(target core.TargetConfig, assessment targetAssessment) core.Artifact {
	components := []core.RepoLegibilityComponent{
		agentDocsComponent(assessment),
		readmeComponent(assessment),
		docAccuracyComponent(assessment),
		contextEconomyComponent(assessment),
		navigabilityComponent(assessment),
	}
	score := 0
	for _, component := range components {
		score += component.Score
	}
	return support.NewRepoLegibilityArtifact(
		"repo_legibility."+artifactSafeID(target.Name, target.Path),
		target.Path,
		core.RepoLegibilityArtifact{Score: score, Components: components},
	)
}

// agentDocsComponent grants 25 points when any agent instruction file exists.
func agentDocsComponent(a targetAssessment) core.RepoLegibilityComponent {
	component := core.RepoLegibilityComponent{Label: "agent_docs", Max: 25}
	if len(a.agentDocs) > 0 {
		component.Score = component.Max
		component.Detail = "found " + strings.Join(a.agentDocs, ", ")
		return component
	}
	component.Detail = "no agent instruction files (CLAUDE.md, AGENTS.md, .cursorrules, .github/copilot-instructions.md)"
	return component
}

// readmeComponent grants 10 points for a root README.md.
func readmeComponent(a targetAssessment) core.RepoLegibilityComponent {
	component := core.RepoLegibilityComponent{Label: "readme", Max: 10, Detail: "README.md missing"}
	if a.readmePresent {
		component.Score = component.Max
		component.Detail = "README.md present"
	}
	return component
}

// docAccuracyComponent starts at 20 and loses 4 points per unresolvable doc
// reference across agent docs and the README.
func docAccuracyComponent(a targetAssessment) core.RepoLegibilityComponent {
	penalty := minInt(20, 4*a.driftReferences)
	return core.RepoLegibilityComponent{
		Label:  "doc_accuracy",
		Score:  20 - penalty,
		Max:    20,
		Detail: fmt.Sprintf("%d unresolvable doc references", a.driftReferences),
	}
}

// contextEconomyComponent scales 25 points by the share of source files that
// blow the context budget; 10% oversized zeroes the component.
func contextEconomyComponent(a targetAssessment) core.RepoLegibilityComponent {
	penalty := 0
	if a.inventory.files > 0 {
		penalty = minInt(25, 25*len(a.inventory.oversized)*10/a.inventory.files)
	}
	return core.RepoLegibilityComponent{
		Label:  "context_economy",
		Score:  25 - penalty,
		Max:    25,
		Detail: fmt.Sprintf("%d of %d source files exceed %d lines", len(a.inventory.oversized), a.inventory.files, a.maxFileLines),
	}
}

// navigabilityComponent scales 20 points by the share of source files caught
// in ambiguous basename groups; 20% affected zeroes the component.
func navigabilityComponent(a targetAssessment) core.RepoLegibilityComponent {
	affected := 0
	for _, group := range a.ambiguousGroups {
		affected += len(group)
	}
	penalty := 0
	if a.inventory.files > 0 {
		penalty = minInt(20, 20*affected*5/a.inventory.files)
	}
	return core.RepoLegibilityComponent{
		Label:  "navigability",
		Score:  20 - penalty,
		Max:    20,
		Detail: fmt.Sprintf("%d files share %d ambiguous basenames", affected, len(a.ambiguousGroups)),
	}
}

func artifactSafeID(name string, fallback string) string {
	value := strings.TrimSpace(name)
	if value == "" {
		value = strings.TrimSpace(fallback)
	}
	replacer := strings.NewReplacer(" ", "-", "/", "-", "\\", "-")
	value = strings.Trim(replacer.Replace(strings.ToLower(value)), "-")
	if value == "" {
		return "target"
	}
	return value
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
