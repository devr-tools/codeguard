package checks_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func qualityOnlyConfig(name string, dir string, language string) codeguard.Config {
	cfg := codeguard.ExampleConfig()
	cfg.Name = name
	cfg.Targets = []codeguard.TargetConfig{{Name: "app", Path: dir, Language: language}}
	cfg.Checks.Quality = true
	cfg.Checks.Security = false
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	return cfg
}

func findingMessagesForRule(report codeguard.Report, ruleID string) []string {
	messages := make([]string, 0)
	for _, section := range report.Sections {
		for _, finding := range section.Findings {
			if finding.RuleID == ruleID {
				messages = append(messages, finding.Message)
			}
		}
	}
	return messages
}

// A def with many parameters inside a docstring used to be reported by the
// line-based scanner; the structured parser must ignore it but still catch
// the real offender.
func TestPythonQualityIgnoresFunctionsInsideStrings(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app.py"), strings.Join([]string{
		"DOC = \"\"\"",
		"def fake(a1, a2, a3, a4, a5, a6, a7, a8, a9):",
		"    pass",
		"\"\"\"",
		"",
		"def real(b1, b2, b3, b4, b5, b6, b7, b8, b9):",
		"    return b1",
		"",
	}, "\n"))

	report, err := codeguard.Run(context.Background(), qualityOnlyConfig("quality-py-strings", dir, "python"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	messages := findingMessagesForRule(report, "quality.max-parameters")
	if len(messages) != 1 {
		t.Fatalf("want exactly one max-parameters finding (real), got %v", messages)
	}
	if !strings.Contains(messages[0], "real") {
		t.Fatalf("finding should name the real function, got %q", messages[0])
	}
}

// A multiline signature used to be missed entirely by the line regex.
func TestPythonQualityCountsMultilineSignatureParameters(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "multi.py"), strings.Join([]string{
		"def wide(",
		"    c1,",
		"    c2,",
		"    c3,",
		"    c4,",
		"    c5,",
		"    c6,",
		"):",
		"    return c1",
		"",
	}, "\n"))

	report, err := codeguard.Run(context.Background(), qualityOnlyConfig("quality-py-multiline", dir, "python"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	messages := findingMessagesForRule(report, "quality.max-parameters")
	if len(messages) != 1 || !strings.Contains(messages[0], "wide") {
		t.Fatalf("multiline signature must be analyzed, got %v", messages)
	}
}

// Functions inside comments or template literals used to register as real
// functions for TypeScript quality metrics.
func TestTypeScriptQualityIgnoresCommentAndTemplateFunctions(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app.ts"), strings.Join([]string{
		"// function fakeComment(a1, a2, a3, a4, a5, a6, a7, a8, a9) { return a1; }",
		"const snippet = `function fakeTemplate(b1, b2, b3, b4, b5, b6, b7, b8, b9) {",
		"  return b1;",
		"}`;",
		"export function real(c1: number, c2: number, c3: number, c4: number, c5: number, c6: number, c7: number, c8: number, c9: number): number {",
		"  return c1;",
		"}",
		"",
	}, "\n"))

	report, err := codeguard.Run(context.Background(), qualityOnlyConfig("quality-ts-strings", dir, "typescript"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	messages := findingMessagesForRule(report, "quality.max-parameters")
	if len(messages) != 1 || !strings.Contains(messages[0], "real") {
		t.Fatalf("want exactly one finding for real, got %v", messages)
	}
}

// Security line rules used to flag shell or eval primitives mentioned in
// comments and string literals.
func TestSecurityIgnoresPrimitivesInCommentsAndStrings(t *testing.T) {
	cases := []struct {
		name     string
		language string
		path     string
		source   string
		ruleID   string
	}{
		{
			name:     "python comment eval",
			language: "python",
			path:     "app.py",
			source:   "# eval('never')\nmessage = \"os.system('fake')\"\n",
			ruleID:   "security.python.dynamic-code",
		},
		{
			name:     "rust comment shell",
			language: "rust",
			path:     "src/lib.rs",
			source:   "// let _ = Command::new(\"sh\");\nconst HELP: &str = \"Command::new(\\\"bash\\\") spawns a shell\";\n",
			ruleID:   "security.rust.shell-execution",
		},
		{
			name:     "java comment exec",
			language: "java",
			path:     "src/Sample.java",
			source:   "class Sample {\n  // Runtime.getRuntime().exec(\"sh\");\n  String doc = \"new ProcessBuilder(cmd)\";\n}\n",
			ruleID:   "security.java.shell-execution",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, filepath.FromSlash(tc.path)), tc.source)

			report, err := codeguard.Run(context.Background(), securityOnlyConfig("security-masking", dir, tc.language))
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if messages := findingMessagesForRule(report, tc.ruleID); len(messages) != 0 {
				t.Fatalf("%s must not flag inside comments/strings, got %v", tc.ruleID, messages)
			}
		})
	}
}
