package agentcontext

import (
	"fmt"
	"path"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// agentDocPaths are the repo-root agent instruction files this family
// recognizes, in the order they are reported.
var agentDocPaths = []string{
	"CLAUDE.md",
	"AGENTS.md",
	".cursorrules",
	".github/copilot-instructions.md",
}

// maxDriftFindingsPerDoc caps drift findings for a single document so one
// badly rotted doc reports a representative sample instead of a wall.
const maxDriftFindingsPerDoc = 20

// presentAgentDocs returns the agent instruction files that exist under the
// target root, in canonical order.
func presentAgentDocs(root string) []string {
	present := make([]string, 0, len(agentDocPaths))
	for _, rel := range agentDocPaths {
		if _, ok := readCappedDocFile(root, rel); ok {
			present = append(present, rel)
		}
	}
	return present
}

func missingAgentDocsFinding(env support.Context) core.Finding {
	return env.NewFinding(support.FindingInput{
		RuleID: "context.agent-docs-missing",
		Level:  "warn",
		Message: "no agent instruction file found (looked for CLAUDE.md, AGENTS.md, .cursorrules, .github/copilot-instructions.md); " +
			"AI agents start every session blind to build, test, and layout conventions — add a CLAUDE.md or AGENTS.md at the repo root",
	})
}

// docResolution is the outcome of resolving one document set's references:
// the drift findings (capped per doc) plus the uncapped broken/total counts
// the legibility score's proportional components are built from.
type docResolution struct {
	findings []core.Finding
	broken   int
	total    int
}

// agentDocsDrift resolves every reference in the present agent docs and
// reports the ones that provably point at nothing.
func agentDocsDrift(env support.Context, resolver *repoResolver, docs []string) docResolution {
	result := docResolution{findings: make([]core.Finding, 0)}
	for _, rel := range docs {
		data, ok := readCappedDocFile(resolver.root, rel)
		if !ok {
			continue
		}
		refs := extractDocReferences(string(data), extractOptions{})
		docResult := driftFindings(env, resolver, rel, refs, "context.agent-docs-drift")
		result.findings = append(result.findings, docResult.findings...)
		result.broken += docResult.broken
		result.total += docResult.total
	}
	return result
}

// readmeDrift applies the same resolver to the root README, scoped to fenced
// shell blocks: the commands a README tells contributors (and agents) to run.
func readmeDrift(env support.Context, resolver *repoResolver) docResolution {
	data, ok := readCappedDocFile(resolver.root, "README.md")
	if !ok {
		return docResolution{}
	}
	refs := extractDocReferences(string(data), extractOptions{commandFencesOnly: true})
	return driftFindings(env, resolver, "README.md", refs, "context.readme-drift")
}

// driftFindings turns the unresolvable subset of refs into findings for one
// document. Findings are capped at maxDriftFindingsPerDoc, but the returned
// broken/total counts always cover every reference so score components stay
// proportional even for badly rotted docs.
func driftFindings(env support.Context, resolver *repoResolver, docRel string, refs []docReference, ruleID string) docResolution {
	result := docResolution{findings: make([]core.Finding, 0), total: len(refs)}
	for _, ref := range refs {
		if resolvable(resolver, docRel, ref) {
			continue
		}
		result.broken++
		if len(result.findings) >= maxDriftFindingsPerDoc {
			continue
		}
		result.findings = append(result.findings, env.NewFinding(support.FindingInput{
			RuleID:  ruleID,
			Level:   "warn",
			Path:    docRel,
			Line:    ref.line,
			Column:  1,
			Message: driftMessage(docRel, ref),
		}))
	}
	return result
}

// agentDocSubstance returns the largest non-blank line count across the
// present agent docs: one substantial CLAUDE.md earns full substance credit
// even when a stub .cursorrules sits next to it.
func agentDocSubstance(root string, docs []string) int {
	best := 0
	for _, rel := range docs {
		data, ok := readCappedDocFile(root, rel)
		if !ok {
			continue
		}
		lines := 0
		for _, line := range strings.Split(string(data), "\n") {
			if strings.TrimSpace(line) != "" {
				lines++
			}
		}
		if lines > best {
			best = lines
		}
	}
	return best
}

// resolvable reports whether a reference resolves. Paths are tried against
// the repo root and, for docs living in a subdirectory, against the doc's own
// directory, so either addressing convention counts as valid.
func resolvable(resolver *repoResolver, docRel string, ref docReference) bool {
	switch ref.kind {
	case refPath:
		if resolver.pathExists(ref.value) {
			return true
		}
		docDir := path.Dir(docRel)
		return docDir != "." && resolver.pathExists(path.Join(docDir, ref.value))
	case refMake:
		return !resolver.makeTargetMissing(ref.value)
	case refNpmScript:
		return !resolver.npmScriptMissing(ref.value)
	default:
		return true
	}
}

func driftMessage(docRel string, ref docReference) string {
	switch ref.kind {
	case refMake:
		return fmt.Sprintf("%s references make target %q, which is not defined in the Makefile; agents follow documented commands literally — update the doc or restore the target", docRel, ref.value)
	case refNpmScript:
		return fmt.Sprintf("%s references npm script %q, which is not defined in package.json; agents follow documented commands literally — update the doc or restore the script", docRel, ref.value)
	default:
		return fmt.Sprintf("%s references %q, which does not exist in the repository; stale paths send agents down dead ends — update or remove the reference", docRel, ref.display)
	}
}
