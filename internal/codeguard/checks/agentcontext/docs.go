package agentcontext

import (
	"fmt"
	"path"

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

// agentDocsDriftFindings resolves every reference in the present agent docs
// and reports the ones that provably point at nothing.
func agentDocsDriftFindings(env support.Context, resolver *repoResolver, docs []string) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, rel := range docs {
		data, ok := readCappedDocFile(resolver.root, rel)
		if !ok {
			continue
		}
		refs := extractDocReferences(string(data), extractOptions{})
		findings = append(findings, driftFindings(env, resolver, rel, refs, "context.agent-docs-drift")...)
	}
	return findings
}

// readmeDriftFindings applies the same resolver to the root README, scoped to
// fenced shell blocks: the commands a README tells contributors (and agents)
// to run.
func readmeDriftFindings(env support.Context, resolver *repoResolver) []core.Finding {
	data, ok := readCappedDocFile(resolver.root, "README.md")
	if !ok {
		return nil
	}
	refs := extractDocReferences(string(data), extractOptions{commandFencesOnly: true})
	return driftFindings(env, resolver, "README.md", refs, "context.readme-drift")
}

// driftFindings turns the unresolvable subset of refs into findings for one
// document.
func driftFindings(env support.Context, resolver *repoResolver, docRel string, refs []docReference, ruleID string) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, ref := range refs {
		if len(findings) >= maxDriftFindingsPerDoc {
			break
		}
		if resolvable(resolver, docRel, ref) {
			continue
		}
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  ruleID,
			Level:   "warn",
			Path:    docRel,
			Line:    ref.line,
			Column:  1,
			Message: driftMessage(docRel, ref),
		}))
	}
	return findings
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
