package checks_test

import (
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// agentContextTestConfig isolates the Agent Context section: every other
// family is disabled so fixtures never trip unrelated checks.
func agentContextTestConfig(dir string, name string) codeguard.Config {
	cfg := codeguard.ExampleConfig()
	cfg.Name = name
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Quality = false
	cfg.Checks.Design = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.SupplyChain = false
	off := false
	cfg.Checks.Contracts = &off
	cfg.Cache.Enabled = &off
	return cfg
}

func agentContextRuleMessages(report codeguard.Report, ruleID string) []string {
	messages := make([]string, 0)
	for _, section := range report.Sections {
		if section.ID != "context" {
			continue
		}
		for _, finding := range section.Findings {
			if finding.RuleID == ruleID {
				messages = append(messages, finding.Message)
			}
		}
	}
	return messages
}

func requireRepoLegibilityArtifact(t *testing.T, report codeguard.Report) codeguard.Artifact {
	t.Helper()
	for _, artifact := range report.Artifacts {
		if artifact.Kind == "repo_legibility" {
			if artifact.RepoLegibility == nil {
				t.Fatal("repo_legibility artifact has no payload")
			}
			return artifact
		}
	}
	t.Fatal("repo_legibility artifact not found")
	return codeguard.Artifact{}
}

func legibilityComponent(t *testing.T, artifact codeguard.Artifact, label string) codeguard.RepoLegibilityComponent {
	t.Helper()
	for _, component := range artifact.RepoLegibility.Components {
		if component.Label == label {
			return component
		}
	}
	t.Fatalf("component %q not found in repo_legibility artifact", label)
	return codeguard.RepoLegibilityComponent{}
}
