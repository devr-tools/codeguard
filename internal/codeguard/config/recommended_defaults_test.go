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
				Reliability: falseValue,
				Data:        falseValue,
				Change:      falseValue,
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
				Reliability:            falseValue,
				Data:                   falseValue,
				Change:                 falseValue,
			},
		},
		{
			name: "keeps opt in sections and scan mode sections unchanged",
			input: core.CheckConfig{
				UseRecommendedDefaults: true,
				Performance:            trueValue,
				SupplyChain:            true,
				Reliability:            trueValue,
				Data:                   trueValue,
				Change:                 trueValue,
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
				Reliability:            trueValue,
				Data:                   trueValue,
				Change:                 trueValue,
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
				Reliability:            trueValue,
				Data:                   trueValue,
				Change:                 trueValue,
				Disabled: []string{
					"quality", "performance", "design", "security", "prompts", "ci", "supply_chain", "reliability", "data", "change", "context", "contracts",
				},
			},
			want: core.CheckConfig{
				UseRecommendedDefaults: true,
				Performance:            falseValue,
				Context:                falseValue,
				Contracts:              falseValue,
				Reliability:            falseValue,
				Data:                   falseValue,
				Change:                 falseValue,
				Disabled: []string{
					"quality", "performance", "design", "security", "prompts", "ci", "supply_chain", "reliability", "data", "change", "context", "contracts",
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
		reflect.DeepEqual(got.Reliability, want.Reliability) &&
		reflect.DeepEqual(got.Data, want.Data) &&
		reflect.DeepEqual(got.Change, want.Change) &&
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
	err = json.Unmarshal(jsonData, &jsonLoaded)
	if err != nil {
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
	err = yaml.Unmarshal(yamlData, &yamlLoaded)
	if err != nil {
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

func TestApplyDefaultsPopulatesChangeRules(t *testing.T) {
	cfg := core.Config{}
	ApplyDefaults(&cfg)

	if cfg.Checks.Change == nil || *cfg.Checks.Change {
		t.Fatalf("default change activation = %v, want explicit false", cfg.Checks.Change)
	}
	if cfg.Checks.ChangeRules.DetectBehaviorChangeWithoutTest == nil || !*cfg.Checks.ChangeRules.DetectBehaviorChangeWithoutTest {
		t.Fatal("expected behavior-change-without-test detector to default on")
	}
	if cfg.Checks.ChangeRules.DetectRefactorBehaviorChange == nil || !*cfg.Checks.ChangeRules.DetectRefactorBehaviorChange {
		t.Fatal("expected refactor behavior-change detector to default on")
	}
	if cfg.Checks.ChangeRules.MaxChangedFiles != 25 {
		t.Fatalf("max changed files = %d, want 25", cfg.Checks.ChangeRules.MaxChangedFiles)
	}
	if cfg.Checks.ChangeRules.MinTestToProductionRatioPercent != 20 {
		t.Fatalf("min test ratio percent = %d, want 20", cfg.Checks.ChangeRules.MinTestToProductionRatioPercent)
	}
}

func TestValidateRejectsInvalidChangeRuleThresholds(t *testing.T) {
	tests := []struct {
		name string
		set  func(*core.Config)
		want string
	}{
		{name: "changed files", set: func(cfg *core.Config) { cfg.Checks.ChangeRules.MaxChangedFiles = -1 }, want: "change_rules.max_changed_files must not be negative"},
		{name: "changed directories", set: func(cfg *core.Config) { cfg.Checks.ChangeRules.MaxChangedDirectories = -1 }, want: "change_rules.max_changed_directories must not be negative"},
		{name: "changed lines", set: func(cfg *core.Config) { cfg.Checks.ChangeRules.MaxChangedLines = -1 }, want: "change_rules.max_changed_lines must not be negative"},
		{name: "public interfaces", set: func(cfg *core.Config) { cfg.Checks.ChangeRules.MaxPublicInterfacesChanged = -1 }, want: "change_rules.max_public_interfaces_changed must not be negative"},
		{name: "concern families", set: func(cfg *core.Config) { cfg.Checks.ChangeRules.MaxConcernFamilies = -1 }, want: "change_rules.max_concern_families must not be negative"},
		{name: "test ratio low", set: func(cfg *core.Config) { cfg.Checks.ChangeRules.MinTestToProductionRatioPercent = -1 }, want: "change_rules.min_test_to_production_ratio_percent must be between 0 and 100"},
		{name: "test ratio high", set: func(cfg *core.Config) { cfg.Checks.ChangeRules.MinTestToProductionRatioPercent = 101 }, want: "change_rules.min_test_to_production_ratio_percent must be between 0 and 100"},
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
