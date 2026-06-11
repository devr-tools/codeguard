package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestSecurityCheckFindsAdditionalLanguagePatterns(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		language string
		path     string
		source   string
		status   string
		ruleIDs  []string
	}{
		{
			name:     "rust",
			language: "rust",
			path:     "src/lib.rs",
			source:   "use std::process::Command;\nlet _client = Client::builder().danger_accept_invalid_certs(true);\nlet _ = Command::new(\"sh\");\n",
			status:   "fail",
			ruleIDs:  []string{"security.rust.insecure-tls", "security.rust.shell-execution"},
		},
		{
			name:     "java",
			language: "java",
			path:     "src/main/java/Sample.java",
			source:   "class Sample {\n  void run() throws Exception {\n    client.setHostnameVerifier((hostname, session) -> true);\n    Runtime.getRuntime().exec(\"sh\");\n  }\n}\n",
			status:   "fail",
			ruleIDs:  []string{"security.java.insecure-tls", "security.java.shell-execution"},
		},
		{
			name:     "csharp",
			language: "csharp",
			path:     "src/Sample.cs",
			source:   "using System.Diagnostics;\nhandler.ServerCertificateCustomValidationCallback = (_, _, _, _) => true;\nProcess.Start(\"cmd.exe\");\n",
			status:   "fail",
			ruleIDs:  []string{"security.csharp.insecure-tls", "security.csharp.shell-execution"},
		},
		{
			name:     "ruby",
			language: "ruby",
			path:     "app/sample.rb",
			source:   "OpenSSL::SSL::VERIFY_NONE\nsystem('ls')\neval('danger')\n",
			status:   "fail",
			ruleIDs:  []string{"security.ruby.insecure-tls", "security.ruby.shell-execution", "security.ruby.dynamic-code"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, filepath.FromSlash(tc.path)), tc.source)

			cfg := codeguard.ExampleConfig()
			cfg.Name = "security-" + tc.name + "-native"
			cfg.Targets = []codeguard.TargetConfig{{Name: tc.name, Path: dir, Language: tc.language}}
			cfg.Checks.Security = true
			cfg.Checks.Design = false
			cfg.Checks.Prompts = false
			cfg.Checks.CI = false
			cfg.Checks.Quality = false
			cfg.Checks.SecurityRules.GovulncheckMode = "off"

			report, err := codeguard.Run(context.Background(), cfg)
			if err != nil {
				t.Fatalf("run: %v", err)
			}

			assertSectionStatus(t, report, "Security", tc.status)
			for _, ruleID := range tc.ruleIDs {
				assertFindingRulePresent(t, report, "Security", ruleID)
			}
		})
	}
}
