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
	agentDocs          []string
	agentDocLines      int
	agentDocBrokenRefs int
	readmePresent      bool
	brokenReferences   int
	totalReferences    int
	inventory          sourceInventory
	ambiguousGroups    [][]string
	maxFileLines       int
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

// agentDocSubstanceLines is the non-blank line count at which an agent
// instruction file earns full substance credit. Ten non-blank lines is
// roughly the minimum for build/test commands plus a layout note; below that
// credit scales linearly, so an empty or one-line CLAUDE.md no longer banks
// the full component.
const agentDocSubstanceLines = 10

// agentDocDriftPenaltyPerRef and agentDocDriftPenaltyCap deduct points from
// the agent_docs component for unresolvable references inside the agent docs
// themselves: a doc full of stale instructions is worse than a short accurate
// one. Capped at 10 so presence plus substance always retains some credit.
const (
	agentDocDriftPenaltyPerRef = 2
	agentDocDriftPenaltyCap    = 10
)

// agentDocsComponent grants up to 25 points for agent instruction files,
// gated on substance and accuracy. Formula (spelled out in Detail):
// substance = 25 x min(non-blank lines, 10)/10 across the largest doc, minus
// 2 points per unresolvable reference in the agent docs (capped at 10),
// floored at 0.
func agentDocsComponent(a targetAssessment) core.RepoLegibilityComponent {
	component := core.RepoLegibilityComponent{Label: "agent_docs", Max: 25}
	if len(a.agentDocs) == 0 {
		component.Detail = "no agent instruction files (CLAUDE.md, AGENTS.md, .cursorrules, .github/copilot-instructions.md)"
		return component
	}
	substance := component.Max * minInt(a.agentDocLines, agentDocSubstanceLines) / agentDocSubstanceLines
	driftPenalty := minInt(agentDocDriftPenaltyCap, agentDocDriftPenaltyPerRef*a.agentDocBrokenRefs)
	driftPenalty = minInt(driftPenalty, substance)
	component.Score = substance - driftPenalty
	component.Detail = fmt.Sprintf(
		"found %s; substance %d/%d (%d non-blank lines, full credit at %d), drift -%d (%d unresolvable references, -%d each capped at %d)",
		strings.Join(a.agentDocs, ", "), substance, component.Max,
		a.agentDocLines, agentDocSubstanceLines,
		driftPenalty, a.agentDocBrokenRefs, agentDocDriftPenaltyPerRef, agentDocDriftPenaltyCap)
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

// docAccuracyComponent scales 20 points by the share of doc references that
// resolve: penalty = round(20 x broken/total). The previous flat -4 per
// broken reference saturated at 5, making 5 broken references indistinguishable
// from 50; the proportional form keeps severity scaling with how much of the
// documentation is wrong, still capped at the full 20 points.
func docAccuracyComponent(a targetAssessment) core.RepoLegibilityComponent {
	component := core.RepoLegibilityComponent{Label: "doc_accuracy", Max: 20}
	if a.totalReferences == 0 {
		component.Score = component.Max
		component.Detail = "no resolvable doc references found"
		return component
	}
	penalty := (component.Max*a.brokenReferences + a.totalReferences/2) / a.totalReferences
	component.Score = component.Max - penalty
	component.Detail = fmt.Sprintf("%d of %d doc references unresolvable (penalty 20 x broken/total)",
		a.brokenReferences, a.totalReferences)
	return component
}

// contextEconomyComponent scales 25 points by the share of source files that
// blow the context budget, ramping linearly to zero at 25% oversized:
// penalty = round(25 x share x 4), capped at 25. The previous x10 multiplier
// zeroed the whole component at 10% oversized, which turned it binary for any
// mature repo carrying a tail of large files; a repo only forfeits the full
// component once one in four files exceeds the budget, while each additional
// oversized file still costs measurably.
func contextEconomyComponent(a targetAssessment) core.RepoLegibilityComponent {
	penalty := 0
	if a.inventory.files > 0 {
		penalty = minInt(25, (100*len(a.inventory.oversized)+a.inventory.files/2)/a.inventory.files)
	}
	return core.RepoLegibilityComponent{
		Label:  "context_economy",
		Score:  25 - penalty,
		Max:    25,
		Detail: fmt.Sprintf("%d of %d source files exceed %d lines (zero credit at 25%% oversized)", len(a.inventory.oversized), a.inventory.files, a.maxFileLines),
	}
}

// navigabilityComponent scales 20 points by the share of source files caught
// in ambiguous basename groups; 20% affected zeroes the component. Groups are
// computed after removing conventional basenames (see
// defaultAmbiguousBasenameIgnore / context_rules.ambiguous_symbol_ignore), so
// language-imposed repeats like index.ts or __init__.py cost nothing.
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
