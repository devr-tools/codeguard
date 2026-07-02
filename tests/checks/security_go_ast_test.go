package checks_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// runGoSecurityScan writes one Go source file and runs a security-only scan
// over it.
func runGoSecurityScan(t *testing.T, name string, sourceLines []string) codeguard.Report {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), strings.Join(sourceLines, "\n")+"\n")

	report, err := codeguard.Run(context.Background(), securityOnlyConfig(name, dir, "go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	return report
}

func TestSecurityGoDetectionPrecision(t *testing.T) {
	cases := []struct {
		name    string
		source  []string
		status  string
		present []string
		absent  []string
	}{
		{
			name: "comment mentioning os/exec and exec.Command does not fire",
			source: []string{
				"package main",
				"",
				"// This package intentionally avoids os/exec; never call exec.Command(\"sh\").",
				"func main() {}",
			},
			status: "pass",
			absent: []string{"security.shell-execution"},
		},
		{
			name: "import of os/exec without a call does not fire",
			source: []string{
				"package main",
				"",
				"import _ \"os/exec\"",
				"",
				"func main() {}",
			},
			status: "pass",
			absent: []string{"security.shell-execution"},
		},
		{
			name: "string literal mentioning risky patterns does not fire",
			source: []string{
				"package main",
				"",
				"const usage = `avoid exec.Command(\"sh\") and InsecureSkipVerify: true in production`",
				"",
				"func main() {}",
			},
			status: "pass",
			absent: []string{"security.shell-execution", "security.insecure-tls"},
		},
		{
			name: "exec.Command call fires",
			source: []string{
				"package main",
				"",
				"import \"os/exec\"",
				"",
				"func main() { _ = exec.Command(\"ls\") }",
			},
			status:  "warn",
			present: []string{"security.shell-execution"},
		},
		{
			name: "aliased exec.CommandContext call fires",
			source: []string{
				"package main",
				"",
				"import (",
				"\t\"context\"",
				"",
				"\trun \"os/exec\"",
				")",
				"",
				"func main() { _ = run.CommandContext(context.Background(), \"ls\") }",
			},
			status:  "warn",
			present: []string{"security.shell-execution"},
		},
		{
			name: "syscall.Exec call fires",
			source: []string{
				"package main",
				"",
				"import \"syscall\"",
				"",
				"func main() { _ = syscall.Exec(\"/bin/ls\", nil, nil) }",
			},
			status:  "warn",
			present: []string{"security.shell-execution"},
		},
		{
			name: "InsecureSkipVerify without space in composite literal fires",
			source: []string{
				"package main",
				"",
				"import \"crypto/tls\"",
				"",
				"func config() *tls.Config { return &tls.Config{InsecureSkipVerify:true} }",
			},
			status:  "fail",
			present: []string{"security.insecure-tls"},
		},
		{
			name: "InsecureSkipVerify assignment fires",
			source: []string{
				"package main",
				"",
				"import \"crypto/tls\"",
				"",
				"func harden(cfg *tls.Config) { cfg.InsecureSkipVerify = true }",
			},
			status:  "fail",
			present: []string{"security.insecure-tls"},
		},
		{
			name: "InsecureSkipVerify set from a non-literal value does not fire",
			source: []string{
				"package main",
				"",
				"import \"crypto/tls\"",
				"",
				"func harden(cfg *tls.Config, allowInsecure bool) { cfg.InsecureSkipVerify = allowInsecure }",
			},
			status: "pass",
			absent: []string{"security.insecure-tls"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report := runGoSecurityScan(t, "security-go-detection", tc.source)
			assertSectionStatus(t, report, "Security", tc.status)
			for _, ruleID := range tc.present {
				assertFindingRulePresent(t, report, "Security", ruleID)
			}
			for _, ruleID := range tc.absent {
				assertFindingRuleAbsent(t, report, "Security", ruleID)
			}
		})
	}
}

func TestSecurityGoFallbackScansMaskedSourceWhenParseFails(t *testing.T) {
	cases := []struct {
		name    string
		source  []string
		status  string
		present []string
		absent  []string
	}{
		{
			name: "shell call in unparseable file still fires",
			source: []string{
				"package main",
				"",
				"func main() {",
				"\texec.Command(\"sh\"", // missing closing paren: file fails to parse
				"}",
			},
			status:  "warn",
			present: []string{"security.shell-execution"},
		},
		{
			name: "insecure TLS without space in unparseable file still fires",
			source: []string{
				"package main",
				"",
				"func broken( {}", // parse error
				"",
				"var cfg = tls.Config{InsecureSkipVerify:true}",
			},
			status:  "fail",
			present: []string{"security.insecure-tls"},
		},
		{
			name: "comment mention in unparseable file does not fire",
			source: []string{
				"package main",
				"",
				"// exec.Command(\"sh\") and InsecureSkipVerify: true are documented here.",
				"func broken( {}", // parse error
			},
			status: "pass",
			absent: []string{"security.shell-execution", "security.insecure-tls"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report := runGoSecurityScan(t, "security-go-fallback", tc.source)
			assertSectionStatus(t, report, "Security", tc.status)
			for _, ruleID := range tc.present {
				assertFindingRulePresent(t, report, "Security", ruleID)
			}
			for _, ruleID := range tc.absent {
				assertFindingRuleAbsent(t, report, "Security", ruleID)
			}
		})
	}
}
