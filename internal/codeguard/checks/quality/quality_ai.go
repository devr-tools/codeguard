package quality

import (
	"go/ast"
	"go/token"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func goAIQualityFindings(env support.Context, file string, fset *token.FileSet, parsed *ast.File, _ []byte) []core.Finding {
	findings := make([]core.Finding, 0)
	ast.Inspect(parsed, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for idx, lhs := range assign.Lhs {
			ident, ok := lhs.(*ast.Ident)
			if !ok || ident.Name != "_" || idx >= len(assign.Rhs) {
				continue
			}
			if rhsIdent, ok := assign.Rhs[idx].(*ast.Ident); ok && rhsIdent.Name == "err" {
				pos := fset.Position(lhs.Pos())
				findings = append(findings, newQualityWarnFinding(env, support.WarnFindingInput{
					RuleID:  "quality.ai.swallowed-error",
					Path:    file,
					Line:    pos.Line,
					Column:  pos.Column,
					Message: "error is assigned to the blank identifier and effectively ignored",
				}))
			}
		}
		return true
	})
	for _, group := range parsed.Comments {
		for _, comment := range group.List {
			text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
			text = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(text, "/*"), "*/"))
			if !isNarrativeComment(text) {
				continue
			}
			pos := fset.Position(comment.Pos())
			findings = append(findings, newQualityWarnFinding(env, support.WarnFindingInput{
				RuleID:  "quality.ai.narrative-comment",
				Path:    file,
				Line:    pos.Line,
				Column:  pos.Column,
				Message: "comment narrates the code instead of explaining intent or constraints",
			}))
		}
	}
	return findings
}

func pythonAIQualityFindings(env support.Context, file string, data []byte) []core.Finding {
	source := strings.ReplaceAll(string(data), "\r\n", "\n")
	findings := make([]core.Finding, 0)
	for _, line := range regexLineMatches(aiPythonPassExceptPattern, source) {
		findings = append(findings, newQualityWarnFinding(env, support.WarnFindingInput{
			RuleID:  "quality.ai.swallowed-error",
			Path:    file,
			Line:    line,
			Column:  1,
			Message: "except block swallows the error without handling or re-raising it",
		}))
	}
	for idx, line := range strings.Split(source, "\n") {
		text := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "#"))
		if !isNarrativeComment(text) {
			continue
		}
		findings = append(findings, newQualityWarnFinding(env, support.WarnFindingInput{
			RuleID:  "quality.ai.narrative-comment",
			Path:    file,
			Line:    idx + 1,
			Column:  1,
			Message: "comment narrates the code instead of explaining intent or constraints",
		}))
	}
	return findings
}

func typeScriptAIQualityFindings(ctx typeScriptScanContext) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, line := range regexLineMatches(aiEmptyCatchPattern, ctx.source) {
		findings = append(findings, newQualityWarnFinding(ctx.env, support.WarnFindingInput{
			RuleID:  "quality.ai.swallowed-error",
			Path:    ctx.file,
			Line:    line,
			Column:  1,
			Message: support.ScriptLabelForPath(ctx.file) + " catch block swallows the error without handling it",
		}))
	}
	inJSDoc := false
	for idx, line := range strings.Split(ctx.source, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "/**") {
			inJSDoc = !strings.Contains(trimmed, "*/")
			continue
		}
		if inJSDoc {
			if strings.Contains(trimmed, "*/") {
				inJSDoc = false
			}
			continue
		}
		text, ok := extractScriptCommentText(line)
		if !ok || !isNarrativeComment(text) {
			continue
		}
		findings = append(findings, newQualityWarnFinding(ctx.env, support.WarnFindingInput{
			RuleID:  "quality.ai.narrative-comment",
			Path:    ctx.file,
			Line:    idx + 1,
			Column:  1,
			Message: support.ScriptLabelForPath(ctx.file) + " comment narrates the code instead of explaining intent or constraints",
		}))
	}
	return findings
}

func maybePutAISlopArtifact(env support.Context, target core.TargetConfig, findings []core.Finding) {
	if env.PutArtifact == nil {
		return
	}
	artifact, ok := aiSlopArtifact(target, findings)
	if !ok {
		return
	}
	recordSlopHistory(env, &artifact)
	env.PutArtifact(artifact)
}

func aiSlopArtifact(target core.TargetConfig, findings []core.Finding) (core.Artifact, bool) {
	components, signals, total, ok := support.WeightedFindingComponents(findings, aiSlopRuleWeights)
	if !ok {
		return core.Artifact{}, false
	}
	language := support.NormalizedLanguage(target.Language)
	if language == "" {
		language = "go"
	}
	return support.NewSlopScoreArtifact(
		"slop_score."+language+"."+support.ArtifactSafeID(target.Name),
		language,
		target.Path,
		core.SlopScoreArtifact{
			Score:      minInt(total*10, 100),
			Signals:    signals,
			Components: components,
		},
	), true
}
