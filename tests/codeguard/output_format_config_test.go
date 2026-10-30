package codeguard_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// Every format the report writer can render must also pass config validation;
// the GitHub Action's PR-comment step silently produced nothing because
// github-comment rendered fine but was rejected before the scan ran.
func TestValidateConfigAcceptsEveryRenderableOutputFormat(t *testing.T) {
	for _, format := range []string{"", "text", "json", "sarif", "github", "github-comment", "GitHub-Comment", "cyclonedx", "cyclonedx-json"} {
		t.Run(format, func(t *testing.T) {
			cfg := codeguard.ExampleConfig()
			cfg.Output.Format = format
			if err := codeguard.ValidateConfig(cfg); err != nil {
				t.Fatalf("ValidateConfig(format=%q) = %v, want nil", format, err)
			}
		})
	}
}

func TestValidateConfigRejectsUnknownOutputFormatAndListsGitHubComment(t *testing.T) {
	cfg := codeguard.ExampleConfig()
	cfg.Output.Format = "markdown"
	err := codeguard.ValidateConfig(cfg)
	if err == nil {
		t.Fatal("ValidateConfig accepted unknown output format \"markdown\"")
	}
	if !strings.Contains(err.Error(), "github-comment") {
		t.Fatalf("error does not list github-comment as a valid format: %v", err)
	}
}
