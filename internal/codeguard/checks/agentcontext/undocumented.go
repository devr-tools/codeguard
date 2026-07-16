package agentcontext

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// highSignalCommandNames is the allowlist of canonical dev-workflow command
// names the undocumented-commands rule cares about. Obscure internal targets
// are deliberately outside the list: not every Makefile target deserves a
// place in the agent docs, but these names are the entrypoints an agent (or a
// new contributor) must be able to discover. Matching is exact — build-ci or
// fmt-check are not high-signal just because they share a prefix.
var highSignalCommandNames = []string{
	"build", "check", "dev", "fmt", "lint", "run", "start", "test",
}

// maxUndocumentedCommandFindings caps this rule's output so a large canonical
// surface reports a representative sample instead of a wall.
const maxUndocumentedCommandFindings = 10

// docContent pairs a doc's repo-relative path with its raw text so mention
// checks can run over every doc that counts as documentation.
type docContent struct {
	rel  string
	text string
}

// undocumentedCommandFindings reports high-signal Makefile targets and
// package.json scripts that no agent doc and not even the root README
// mentions — the inverse of drift: the command exists, the docs are silent.
// When the repo has no agent docs at all the rule stays silent;
// context.agent-docs-missing already covers that hole.
func undocumentedCommandFindings(env support.Context, resolver *repoResolver, agentDocs []string) []core.Finding {
	if len(agentDocs) == 0 {
		return nil
	}
	docs := loadDocContents(resolver.root, append(append([]string{}, agentDocs...), "README.md"))
	documented := documentedCommandSet(docs)
	findings := make([]core.Finding, 0)
	for _, name := range highSignalCommandNames {
		if len(findings) >= maxUndocumentedCommandFindings {
			return findings
		}
		if canonicalMakeTarget(resolver, name) && !commandDocumented(docs, documented, refMake, name) {
			findings = append(findings, undocumentedCommandFinding(env, "Makefile", "make "+name))
		}
	}
	for _, name := range highSignalCommandNames {
		if len(findings) >= maxUndocumentedCommandFindings {
			return findings
		}
		if canonicalNpmScript(resolver, name) && !commandDocumented(docs, documented, refNpmScript, name) {
			findings = append(findings, undocumentedCommandFinding(env, "package.json", "npm run "+name))
		}
	}
	return findings
}

// canonicalMakeTarget reports whether name is a target the root Makefile
// provably defines. Unreliable Makefiles (includes, pattern rules) yield no
// canonical set — the same conservative stance the drift rules take.
func canonicalMakeTarget(resolver *repoResolver, name string) bool {
	if !resolver.makeReliable {
		return false
	}
	_, ok := resolver.makeTargets[name]
	return ok
}

// canonicalNpmScript reports whether name is a script the root package.json
// provably defines; workspace roots yield no canonical set.
func canonicalNpmScript(resolver *repoResolver, name string) bool {
	if !resolver.npmReliable {
		return false
	}
	_, ok := resolver.npmScripts[name]
	return ok
}

// loadDocContents reads each existing doc once for the mention checks.
func loadDocContents(root string, rels []string) []docContent {
	docs := make([]docContent, 0, len(rels))
	for _, rel := range rels {
		if data, ok := readCappedDocFile(root, rel); ok {
			docs = append(docs, docContent{rel: rel, text: string(data)})
		}
	}
	return docs
}

// documentedCommandSet collects every make target and npm script the docs
// reference through the structured extractor (inline code and shell fences).
func documentedCommandSet(docs []docContent) map[string]struct{} {
	documented := map[string]struct{}{}
	for _, doc := range docs {
		for _, ref := range extractDocReferences(doc.text) {
			if ref.kind == refMake || ref.kind == refNpmScript {
				documented[string(ref.kind)+"|"+ref.value] = struct{}{}
			}
		}
	}
	return documented
}

// commandDocumented reports whether any doc mentions the command. The stance
// is the mirror image of drift's precision rule: a finding here claims the
// docs are silent, so ANY plausible mention counts — a structured reference
// from the extractor, or a plain-text invocation anywhere in the doc
// (prose without backticks, non-shell fences, captured output).
func commandDocumented(docs []docContent, documented map[string]struct{}, kind refKind, name string) bool {
	if _, ok := documented[string(kind)+"|"+name]; ok {
		return true
	}
	mention := commandMentionPattern(kind, name)
	for _, doc := range docs {
		if mention.MatchString(doc.text) {
			return true
		}
	}
	return false
}

// commandMentionPattern matches a literal invocation of the command with the
// same word-boundary alphabet isPlainCommandWord accepts, so `make fmt-check`
// never counts as a mention of `make fmt`.
func commandMentionPattern(kind refKind, name string) *regexp.Regexp {
	const boundary = `[^A-Za-z0-9._/:-]`
	quoted := regexp.QuoteMeta(name)
	switch kind {
	case refNpmScript:
		return regexp.MustCompile(`(?m)(?:^|` + boundary + `)(?:npm|pnpm|yarn)[ \t]+(?:run(?:-script)?[ \t]+)?` + quoted + `(?:$|` + boundary + `)`)
	default:
		return regexp.MustCompile(`(?m)(?:^|` + boundary + `)make[ \t]+` + quoted + `(?:$|` + boundary + `)`)
	}
}

func undocumentedCommandFinding(env support.Context, path string, invocation string) core.Finding {
	return env.NewFinding(support.FindingInput{
		RuleID: "context.undocumented-commands",
		Level:  "warn",
		Path:   path,
		Line:   1,
		Column: 1,
		Message: fmt.Sprintf("canonical dev command %q is not mentioned in any agent instruction file or the root README; "+
			"agents cannot discover entrypoints the docs never name — document it in CLAUDE.md/AGENTS.md or the README", strings.TrimSpace(invocation)),
	})
}
