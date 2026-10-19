package codeguard_test

import (
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// The starter config that `codeguard init` writes used to double as the source
// of built-in defaults, so every convention written for the starter — a
// workflow filename, a `go test ./...` marker, a `tests/**` test layout — was
// silently the effective policy for anyone who omitted that key. The two roles
// are now separate: ApplyDefaults resolves against a conservative baseline that
// asserts no repository shape at all.
//
// Each list below is one that its rule treats as "not configured" when empty,
// so an empty default means "mechanism intact, nothing enforced". Do not make
// any of them non-empty to express codeguard's own conventions; put those in
// the starter config (config.applyStarterPolicy) or in a named profile.
func TestDefaultsAssertNoRepositoryShapePolicy(t *testing.T) {
	cfg := codeguard.Config{}
	codeguard.ApplyDefaults(&cfg)

	ci := cfg.Checks.CIRules
	for _, tc := range []struct {
		key    string
		values []string
	}{
		{"ci_rules.allowed_test_paths", ci.AllowedTestPaths},
		{"ci_rules.required_workflow_files", ci.RequiredWorkflowFiles},
		{"ci_rules.required_release_files", ci.RequiredReleaseFiles},
		{"ci_rules.required_automation_paths", ci.RequiredAutomationPaths},
		{"ci_rules.required_gates", ci.RequiredGates},
	} {
		if len(tc.values) != 0 {
			t.Errorf("default %s = %v, want empty: a default here enforces codeguard's own repository layout on every user", tc.key, tc.values)
		}
	}
	if len(ci.WorkflowContentRules) != 0 {
		t.Errorf("default ci_rules.workflow_content_rules = %v, want empty", ci.WorkflowContentRules)
	}

	design := cfg.Checks.DesignRules
	for _, tc := range []struct {
		key   string
		value *bool
	}{
		{"design_rules.require_cmd_through_internal_cli", design.RequireCmdThroughInternalCLI},
		{"design_rules.forbid_internal_import_cmd", design.ForbidInternalImportCmd},
		{"design_rules.forbid_service_import_internal", design.ForbidServiceImportInternal},
		{"design_rules.forbid_service_import_cmd", design.ForbidServiceImportCmd},
	} {
		// The toggles must still be resolved (the checks dereference them),
		// just resolved to off.
		if tc.value == nil {
			t.Errorf("default %s is nil; the design checks dereference it", tc.key)
			continue
		}
		if *tc.value {
			t.Errorf("default %s = true, want false: it encodes one specific repository layout at fail level", tc.key)
		}
	}
}

// The starter config keeps stating codeguard's conventions: the split must not
// be achieved by thinning out what `codeguard init` writes.
func TestStarterConfigStillStatesHouseConventions(t *testing.T) {
	cfg := codeguard.ExampleConfig()

	if got := cfg.Checks.CIRules.AllowedTestPaths; len(got) == 0 {
		t.Error("starter config should still state a test layout")
	}
	if got := cfg.Checks.CIRules.RequiredWorkflowFiles; len(got) == 0 {
		t.Error("starter config should still state required workflow files")
	}
	if got := cfg.Checks.CIRules.WorkflowContentRules; len(got) == 0 {
		t.Error("starter config should still state workflow content rules")
	}
	if cfg.Checks.DesignRules.RequireCmdThroughInternalCLI == nil || !*cfg.Checks.DesignRules.RequireCmdThroughInternalCLI {
		t.Error("starter config should still state the cmd/ layering rule")
	}
	if !cfg.Checks.Quality || !cfg.Checks.Design || !cfg.Checks.Security || !cfg.Checks.Prompts || !cfg.Checks.CI {
		t.Errorf("starter config should enable the recommended sections, got %+v", cfg.Checks)
	}
	if len(cfg.Targets) == 0 {
		t.Error("starter config should ship a target")
	}
}
