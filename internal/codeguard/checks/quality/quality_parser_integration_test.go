package quality

import (
	"bytes"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func TestParserBackedLanguagesFeedMaintainabilityFindings(t *testing.T) {
	env := support.Context{
		Config: core.Config{
			Checks: core.CheckConfig{
				QualityRules: core.QualityRulesConfig{
					MaxFileLines:            100,
					MaxFunctionLines:        3,
					MaxParameters:           2,
					MaxCyclomaticComplexity: 2,
				},
			},
		},
		CountLines: func(data []byte) int {
			return bytes.Count(data, []byte{'\n'}) + 1
		},
		NewFinding: func(input support.FindingInput) core.Finding {
			return core.Finding{
				RuleID:  input.RuleID,
				Level:   input.Level,
				Message: input.Message,
				Path:    input.Path,
				Line:    input.Line,
				Column:  input.Column,
			}
		},
	}

	testCases := []struct {
		name string
		run  func() []core.Finding
	}{
		{
			name: "python",
			run: func() []core.Finding {
				source := []byte(`def build(
    a,
    /,
    b,
    *,
    c,
):
    if a and b:
        return c
    return b
`)
				return pythonFindingsForFile(env, "pkg/example.py", source)
			},
		},
		{
			name: "rust",
			run: func() []core.Finding {
				source := []byte(`impl Example {
    fn build(&self, left: Vec<(String, String)>, right: Vec<String>, flag: bool) -> bool {
        if flag && left.is_empty() {
            return false;
        }
        right.is_empty()
    }
}
`)
				return rustFindingsForFile(env, "pkg/example.rs", source)
			},
		},
		{
			name: "java",
			run: func() []core.Finding {
				source := []byte(`class Example {
    @Route(path = "/x")
    public String build(String left, java.util.Map<String, Integer> right, boolean flag) {
        if (flag || right.isEmpty()) {
            return left;
        }
        return left;
    }
}
`)
				return javaFindingsForFile(env, "pkg/Example.java", source)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			findings := tc.run()
			if len(findings) != 3 {
				t.Fatalf("expected 3 findings, got %d", len(findings))
			}
			assertHasRule(t, findings, "quality.max-function-lines")
			assertHasRule(t, findings, "quality.max-parameters")
			assertHasRule(t, findings, "quality.cyclomatic-complexity")
		})
	}
}

func assertHasRule(t *testing.T, findings []core.Finding, ruleID string) {
	t.Helper()
	for _, finding := range findings {
		if finding.RuleID == ruleID {
			return
		}
	}
	t.Fatalf("expected finding %q, got %#v", ruleID, findings)
}
