package config

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"gopkg.in/yaml.v3"
)

func TestApplyDefaultsRecommendedDefaultsResolution(t *testing.T) {
	trueValue := true
	falseValue := false

	for _, tt := range recommendedDefaultsCases(&trueValue, &falseValue) {
		t.Run(tt.name, func(t *testing.T) {
			cfg := core.Config{Checks: tt.input}
			ApplyDefaults(&cfg)

			if got := cfg.Checks; !sameCheckActivation(got, tt.want) {
				t.Fatalf("check activation after ApplyDefaults = %#v, want %#v", got, tt.want)
			}

			again := cfg
			ApplyDefaults(&again)
			if !sameCheckActivation(again.Checks, cfg.Checks) {
				t.Fatalf("ApplyDefaults is not idempotent: first %#v, second %#v", cfg.Checks, again.Checks)
			}
		})
	}
}

type recommendedDefaultsCase struct {
	name  string
	input core.CheckConfig
	want  core.CheckConfig
}

func recommendedDefaultsCases(trueValue, falseValue *bool) []recommendedDefaultsCase {
	return []recommendedDefaultsCase{
		{
			name: "preserves legacy section settings without opt in",
			input: core.CheckConfig{
				Performance: trueValue,
				SupplyChain: true,
				Context:     falseValue,
				Contracts:   trueValue,
			},
			want: core.CheckConfig{
				Performance: trueValue,
				SupplyChain: true,
				Context:     falseValue,
				Contracts:   trueValue,
			},
		},
		{
			name: "enables exactly the recommended baseline",
			input: core.CheckConfig{
				UseRecommendedDefaults: true,
			},
			want: core.CheckConfig{
				UseRecommendedDefaults: true,
				Quality:                true,
				Design:                 true,
				Security:               true,
				Prompts:                true,
				CI:                     true,
			},
		},
		{
			name: "keeps opt in sections and scan mode sections unchanged",
			input: core.CheckConfig{
				UseRecommendedDefaults: true,
				Performance:            trueValue,
				SupplyChain:            true,
			},
			want: core.CheckConfig{
				UseRecommendedDefaults: true,
				Quality:                true,
				Design:                 true,
				Security:               true,
				Prompts:                true,
				CI:                     true,
				Performance:            trueValue,
				SupplyChain:            true,
			},
		},
		{
			name: "disabled sections take final precedence",
			input: core.CheckConfig{
				UseRecommendedDefaults: true,
				Performance:            trueValue,
				SupplyChain:            true,
				Context:                trueValue,
				Contracts:              trueValue,
				Disabled: []string{
					"quality", "performance", "design", "security", "prompts", "ci", "supply_chain", "context", "contracts",
				},
			},
			want: core.CheckConfig{
				UseRecommendedDefaults: true,
				Performance:            falseValue,
				Context:                falseValue,
				Contracts:              falseValue,
				Disabled: []string{
					"quality", "performance", "design", "security", "prompts", "ci", "supply_chain", "context", "contracts",
				},
			},
		},
	}
}

func sameCheckActivation(got, want core.CheckConfig) bool {
	return got.UseRecommendedDefaults == want.UseRecommendedDefaults &&
		reflect.DeepEqual(got.Disabled, want.Disabled) &&
		got.Quality == want.Quality &&
		got.Design == want.Design &&
		got.Security == want.Security &&
		got.Prompts == want.Prompts &&
		got.CI == want.CI &&
		got.SupplyChain == want.SupplyChain &&
		reflect.DeepEqual(got.Performance, want.Performance) &&
		reflect.DeepEqual(got.Context, want.Context) &&
		reflect.DeepEqual(got.Contracts, want.Contracts)
}

func TestCheckConfigRecommendedDefaultsSerialization(t *testing.T) {
	want := core.CheckConfig{
		UseRecommendedDefaults: true,
		Disabled:               []string{"quality", "contracts"},
	}

	jsonData, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal JSON: %v", err)
	}
	if !strings.Contains(string(jsonData), `"use_recommended_defaults":true`) || !strings.Contains(string(jsonData), `"disabled":["quality","contracts"]`) {
		t.Fatalf("JSON did not preserve recommended defaults fields: %s", jsonData)
	}
	var jsonLoaded core.CheckConfig
	if err := json.Unmarshal(jsonData, &jsonLoaded); err != nil {
		t.Fatalf("unmarshal JSON: %v", err)
	}
	if !reflect.DeepEqual(jsonLoaded, want) {
		t.Fatalf("JSON round trip = %#v, want %#v", jsonLoaded, want)
	}

	yamlData, err := yaml.Marshal(want)
	if err != nil {
		t.Fatalf("marshal YAML: %v", err)
	}
	if !strings.Contains(string(yamlData), "use_recommended_defaults: true") || !strings.Contains(string(yamlData), "disabled:\n") {
		t.Fatalf("YAML did not preserve recommended defaults fields: %s", yamlData)
	}
	var yamlLoaded core.CheckConfig
	if err := yaml.Unmarshal(yamlData, &yamlLoaded); err != nil {
		t.Fatalf("unmarshal YAML: %v", err)
	}
	if !reflect.DeepEqual(yamlLoaded, want) {
		t.Fatalf("YAML round trip = %#v, want %#v", yamlLoaded, want)
	}
}

func TestValidateRejectsInvalidDisabledChecks(t *testing.T) {
	tests := []struct {
		name string
		list []string
		want string
	}{
		{name: "blank", list: []string{" "}, want: "checks.disabled[0] must not be blank"},
		{name: "duplicate", list: []string{"quality", "quality"}, want: `checks.disabled contains duplicate section "quality"`},
		{name: "unknown", list: []string{"supply-chain"}, want: `checks.disabled contains unknown section "supply-chain"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ExampleConfig()
			cfg.Checks.Disabled = tt.list

			err := Validate(cfg)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestValidateRejectsNegativeBasicThresholds(t *testing.T) {
	tests := []struct {
		name string
		set  func(*core.Config)
		want string
	}{
		{name: "quality max file lines", set: func(cfg *core.Config) { cfg.Checks.QualityRules.MaxFileLines = -1 }, want: "quality_rules.max_file_lines must not be negative"},
		{name: "quality max function lines", set: func(cfg *core.Config) { cfg.Checks.QualityRules.MaxFunctionLines = -1 }, want: "quality_rules.max_function_lines must not be negative"},
		{name: "quality max parameters", set: func(cfg *core.Config) { cfg.Checks.QualityRules.MaxParameters = -1 }, want: "quality_rules.max_parameters must not be negative"},
		{name: "quality cyclomatic complexity", set: func(cfg *core.Config) { cfg.Checks.QualityRules.MaxCyclomaticComplexity = -1 }, want: "quality_rules.max_cyclomatic_complexity must not be negative"},
		{name: "quality clone token threshold", set: func(cfg *core.Config) { cfg.Checks.QualityRules.CloneTokenThreshold = -1 }, want: "quality_rules.clone_token_threshold must not be negative"},
		{name: "design max declarations", set: func(cfg *core.Config) { cfg.Checks.DesignRules.MaxDeclsPerFile = -1 }, want: "design_rules.max_decls_per_file must not be negative"},
		{name: "design max methods", set: func(cfg *core.Config) { cfg.Checks.DesignRules.MaxMethodsPerType = -1 }, want: "design_rules.max_methods_per_type must not be negative"},
		{name: "design max interface methods", set: func(cfg *core.Config) { cfg.Checks.DesignRules.MaxInterfaceMethods = -1 }, want: "design_rules.max_interface_methods must not be negative"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ExampleConfig()
			tt.set(&cfg)

			err := Validate(cfg)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %v, want %q", err, tt.want)
			}
		})
	}
}
