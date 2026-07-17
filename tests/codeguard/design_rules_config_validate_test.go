package codeguard_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestValidateDesignArchitectureRules(t *testing.T) {
	tests := []struct {
		name  string
		rules codeguard.DesignRulesConfig
		want  string
	}{
		{
			name: "unknown layer dependency",
			rules: codeguard.DesignRulesConfig{Layers: []codeguard.DesignLayerConfig{{
				Name: "domain", Paths: []string{"domain/**"}, MayDependOn: []string{"missing"},
			}}},
			want: "unknown layer",
		},
		{
			name: "duplicate domain",
			rules: codeguard.DesignRulesConfig{Domains: []codeguard.DesignDomainConfig{
				{Name: "orders", Paths: []string{"orders/**"}},
				{Name: "orders", Paths: []string{"other/**"}},
			}},
			want: "duplicate name",
		},
		{
			name: "incomplete capability",
			rules: codeguard.DesignRulesConfig{Capabilities: []codeguard.DesignCapabilityConfig{{
				Name: "database", Imports: []string{"database/sql"},
			}}},
			want: "allowed_paths",
		},
		{
			name: "incomplete public surface",
			rules: codeguard.DesignRulesConfig{PublicSurfaces: []codeguard.DesignPublicSurfaceConfig{{
				Name: "billing", Paths: []string{"billing/**"},
			}}},
			want: "entrypoints",
		},
		{
			name: "incomplete production test policy",
			rules: codeguard.DesignRulesConfig{ProductionTest: &codeguard.DesignProductionTestConfig{
				ProductionPaths: []string{"src/**"},
			}},
			want: "test_paths",
		},
		{
			name: "invalid stability delta",
			rules: codeguard.DesignRulesConfig{Stability: &codeguard.DesignStabilityConfig{
				MaxInstabilityDelta: 1.1,
			}},
			want: "between 0 and 1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDesignRulesConfig(tt.rules)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestValidateDesignArchitectureRulesRejectsCaseInsensitiveDuplicates(t *testing.T) {
	rules := codeguard.DesignRulesConfig{Layers: []codeguard.DesignLayerConfig{
		{Name: "Domain", Paths: []string{"internal/domain/**"}},
		{Name: "domain", Paths: []string{"src/domain/**"}},
	}}
	if err := validateDesignRulesConfig(rules); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("error = %v, want case-insensitive duplicate rejection", err)
	}
}

func TestValidateDesignArchitectureRulesAcceptsCompletePolicy(t *testing.T) {
	rules := codeguard.DesignRulesConfig{
		Layers: []codeguard.DesignLayerConfig{
			{Name: "domain", Paths: []string{"domain/**"}, MayDependOn: []string{"domain"}},
			{Name: "adapters", Paths: []string{"adapters/**"}, MayDependOn: []string{"domain"}},
		},
		Domains: []codeguard.DesignDomainConfig{
			{Name: "shared", Paths: []string{"shared/**"}, PublicPaths: []string{"shared/contracts/**"}},
			{Name: "orders", Paths: []string{"orders/**"}, MayDependOn: []string{"shared"}},
		},
		Capabilities:   []codeguard.DesignCapabilityConfig{{Name: "database", Imports: []string{"database/sql"}, AllowedPaths: []string{"adapters/**"}}},
		PublicSurfaces: []codeguard.DesignPublicSurfaceConfig{{Name: "orders", Paths: []string{"orders/**"}, Entrypoints: []string{"orders/index.ts"}}},
		ProductionTest: &codeguard.DesignProductionTestConfig{ProductionPaths: []string{"src/**"}, TestPaths: []string{"test/**"}},
		Reachability:   &codeguard.DesignReachabilityConfig{Entrypoints: []string{"src/main.ts"}},
		Stability:      &codeguard.DesignStabilityConfig{MinimumFanIn: 3, MaxInstabilityDelta: 0.4},
	}
	if err := validateDesignRulesConfig(rules); err != nil {
		t.Fatalf("validate complete policy: %v", err)
	}
}

func validateDesignRulesConfig(rules codeguard.DesignRulesConfig) error {
	cfg := codeguard.ExampleConfig()
	cfg.Checks.DesignRules = rules
	return codeguard.ValidateConfig(cfg)
}
