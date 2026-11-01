package rules_test

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/config"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/internal/codeguard/rules"
)

// TestEveryCatalogRuleMapsToAKnownSection keeps the display-section mapping
// complete: `rules -section`, `rules -enabled`, and explain's section toggle
// all depend on it, so a new display section must be mapped when it lands.
func TestEveryCatalogRuleMapsToAKnownSection(t *testing.T) {
	for id, meta := range rules.Catalog() {
		key := rules.SectionKey(meta)
		if key == "" || !core.KnownSectionKey(key) {
			t.Errorf("%s: display section %q maps to %q, not a known section id", id, meta.Section, key)
		}
		if toggle := rules.SectionToggleKey(key); toggle == "" || len(meta.ConfigKeys) == 0 || meta.ConfigKeys[0].Key != toggle {
			t.Errorf("%s: config keys %v should start with the section toggle %q", id, meta.ConfigKeys, toggle)
		}
	}
}

// TestEveryRuleConfigKeyExistsInConfigStructs walks core.Config by yaml tags
// independently of the config package, so a renamed or removed field cannot
// leave explain pointing at a key that no longer exists. Enable keys must
// also be booleans, since explain and `rules -enabled` evaluate them as such.
func TestEveryRuleConfigKeyExistsInConfigStructs(t *testing.T) {
	for id, meta := range rules.Catalog() {
		for _, key := range meta.ConfigKeys {
			fieldType, ok := yamlFieldType(reflect.TypeOf(core.Config{}), key.Key)
			if !ok {
				t.Errorf("%s: config key %q does not exist in the config structs", id, key.Key)
				continue
			}
			if !config.KnownConfigKey(key.Key) {
				t.Errorf("%s: config.KnownConfigKey(%q) = false, but the yaml walk found it", id, key.Key)
			}
			switch key.Role {
			case core.RuleConfigKeyRoleEnable:
				if derefType(fieldType).Kind() != reflect.Bool {
					t.Errorf("%s: enable key %q has type %s, want bool", id, key.Key, fieldType)
				}
			case core.RuleConfigKeyRoleTune:
			default:
				t.Errorf("%s: config key %q has unknown role %q", id, key.Key, key.Role)
			}
		}
	}
}

func yamlFieldType(root reflect.Type, key string) (reflect.Type, bool) {
	current := root
	for _, segment := range strings.Split(key, ".") {
		current = derefType(current)
		if current.Kind() != reflect.Struct {
			return nil, false
		}
		found := false
		for i := 0; i < current.NumField(); i++ {
			field := current.Field(i)
			if strings.Split(field.Tag.Get("yaml"), ",")[0] == segment {
				current, found = field.Type, true
				break
			}
		}
		if !found {
			return nil, false
		}
	}
	return current, true
}

func derefType(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}

// TestRequiredRuleFamiliesHaveExamples pins the example coverage explain
// promises: every security taint rule, every prompts rule (prompt governance
// plus agent-config and MCP rules), every ci.workflow-* security rule, and
// the quality and design rules codeguard reports most often on real repos.
func TestRequiredRuleFamiliesHaveExamples(t *testing.T) {
	catalog := rules.Catalog()
	required := []string{
		"function.inconsistent-return-contract", "function.hidden-mutation", "quality.mutable-global-state",
		"design.max-decls-per-file", "quality.boolean-argument", "naming.boolean-not-predicate",
		"error.context-lost", "quality.cyclomatic-complexity", "function.mixed-abstraction-level",
		"function.command-query-mix", "defensive.unvalidated-boundary-input", "error.partial-failure-hidden",
		"defensive.missing-schema-validation", "defensive.invalid-state-representable", "defensive.integer-overflow",
		"defensive.bounds-assumption", "defensive.unchecked-type-assertion", "quality.max-parameters",
		"function.excessive-parameters", "quality.max-file-lines",
	}
	for id := range catalog {
		isTaint := strings.HasPrefix(id, "security.taint.") || strings.HasPrefix(id, "security.ssrf.") || strings.HasSuffix(id, ".taint-flow")
		isWorkflow := strings.HasPrefix(id, "ci.workflow-") && id != "ci.workflow-content"
		if isTaint || isWorkflow || strings.HasPrefix(id, "prompts.") {
			required = append(required, id)
		}
	}
	for _, id := range required {
		meta, ok := catalog[id]
		if !ok {
			t.Errorf("required rule %s is not in the catalog", id)
			continue
		}
		if len(meta.Examples) == 0 {
			t.Errorf("%s has no examples", id)
		}
	}
	if len(required) < 44 {
		t.Fatalf("only %d rules require examples; the family selection is too narrow", len(required))
	}
}

func TestRuleDocsAnchorScheme(t *testing.T) {
	if got := rules.RuleDocsAnchor("security.taint.go"); got != "rule-security-taint-go" {
		t.Fatalf("anchor = %q", got)
	}
	if got := rules.RuleDocsPath("ci.workflow-script-injection"); got != "docs/checks.md#rule-ci-workflow-script-injection" {
		t.Fatalf("docs path = %q", got)
	}
}

func TestAliasesForReturnsRetiredIDs(t *testing.T) {
	if got := rules.AliasesFor("function.mixed-abstraction-level"); !slices.Contains(got, "quality.mixed-abstraction-levels") {
		t.Fatalf("aliases = %v", got)
	}
	if got := rules.AliasesFor("security.taint.go"); len(got) != 0 {
		t.Fatalf("aliases = %v, want none", got)
	}
}

func TestRuleProfileNames(t *testing.T) {
	catalog := rules.Catalog()
	if got := config.RuleProfileNames(catalog["security.taint.go"]); !slices.Equal(got, []string{"default", "ai-safe", "enterprise", "startup", "strict"}) {
		t.Fatalf("security.taint.go profiles = %v", got)
	}
	if got := config.RuleProfileNames(catalog["reliability.missing-timeout"]); !slices.Equal(got, []string{"ai-safe", "enterprise", "strict"}) {
		t.Fatalf("reliability.missing-timeout profiles = %v", got)
	}
	if got := config.RuleProfileNames(catalog["supply_chain.missing-lockfile"]); !slices.Equal(got, []string{"enterprise"}) {
		t.Fatalf("supply_chain.missing-lockfile profiles = %v", got)
	}
}

func TestRuleEnabledHonoursRuleLevelSwitches(t *testing.T) {
	meta := rules.Catalog()["function.excessive-parameters"]
	cfg := core.Config{Checks: core.CheckConfig{Quality: true}}
	config.ApplyDefaults(&cfg)
	if !config.RuleEnabled(cfg, meta) {
		t.Fatal("quality on with default local_precision should enable function.excessive-parameters")
	}
	off := false
	cfg.Checks.QualityRules.LocalPrecision = &off
	if config.RuleEnabled(cfg, meta) {
		t.Fatal("local_precision=false should disable function.excessive-parameters")
	}
	cfg.Checks.QualityRules.LocalPrecision = nil
	cfg.Checks.Quality = false
	if config.RuleEnabled(cfg, meta) {
		t.Fatal("checks.quality=false should disable function.excessive-parameters")
	}
}

func TestLookupConfigKey(t *testing.T) {
	cfg := core.Config{}
	config.ApplyDefaults(&cfg)
	if value, ok := config.LookupConfigKey(cfg, "checks.quality_rules.max_parameters"); !ok || value != 5 {
		t.Fatalf("max_parameters = %v, %v", value, ok)
	}
	if value, ok := config.LookupConfigKey(cfg, "checks.context"); !ok || value != nil {
		t.Fatalf("checks.context = %v, %v; want unset", value, ok)
	}
	if _, ok := config.LookupConfigKey(cfg, "checks.no_such_key"); ok {
		t.Fatal("unknown key reported as present")
	}
}
