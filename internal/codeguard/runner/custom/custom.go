package custom

import (
	"context"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/ai/nlrule"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

func RunSection(ctx context.Context, sc runnersupport.Context) core.SectionResult {
	findings := make([]core.Finding, 0)
	for _, target := range sc.Cfg.Targets {
		findings = append(findings, runnersupport.ScanTargetFiles(sc, target, "custom", func(string) bool { return true }, func(file string, data []byte) []core.Finding {
			localFindings := make([]core.Finding, 0)
			lines := strings.Split(string(data), "\n")
			for _, rule := range sc.CustomRules {
				if !rule.MatchesPath(file) {
					continue
				}
				if rule.UsesNaturalLanguage() {
					matches, err := nlrule.EvaluateFileCached(ctx, sc.NLRuntime, sc.Cache, nlrule.FileEvaluation{Rule: rule.Rule, Path: file, Data: data})
					if err != nil {
						continue
					}
					for _, match := range matches {
						localFindings = append(localFindings, runnersupport.NewFinding(sc, runnersupport.FindingInput{
							RuleID:  rule.Rule.ID,
							Level:   runnersupport.NormalizedSeverity(rule.Rule.Severity),
							Path:    file,
							Line:    match.Line,
							Column:  match.Column,
							Message: match.Message,
							Why:     match.Why,
						}))
					}
					continue
				}
				if rule.ContentRegex == nil {
					localFindings = append(localFindings, runnersupport.NewFinding(sc, runnersupport.FindingInput{
						RuleID:  rule.Rule.ID,
						Level:   runnersupport.NormalizedSeverity(rule.Rule.Severity),
						Path:    file,
						Message: rule.Rule.Message,
					}))
					continue
				}
				for idx, line := range lines {
					if rule.ContentRegex.MatchString(line) {
						localFindings = append(localFindings, runnersupport.NewFinding(sc, runnersupport.FindingInput{
							RuleID:  rule.Rule.ID,
							Level:   runnersupport.NormalizedSeverity(rule.Rule.Severity),
							Path:    file,
							Line:    idx + 1,
							Column:  1,
							Message: rule.Rule.Message,
						}))
					}
				}
			}
			return localFindings
		})...)
	}
	return runnersupport.FinalizeSection(sc, "custom", "Custom Rules", findings)
}
