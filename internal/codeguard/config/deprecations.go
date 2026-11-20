package config

import (
	"fmt"
	"strconv"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"gopkg.in/yaml.v3"
)

// DeprecationRemovalVersion is the release in which deprecated config keys
// stop being accepted.
const DeprecationRemovalVersion = "2.0"

// Deprecation reports one deprecated key that a config file sets. Deprecated
// keys still parse, so 1.x config files keep loading, but codeguard ignores
// their values.
type Deprecation struct {
	// Key is the dotted path of the key as written in the file, with list
	// indexes, for example rule_packs[0].rules[2].ai_prompt.
	Key string `json:"key"`
	// File is the file that sets the key.
	File string `json:"file,omitempty"`
	// Reason explains why the key has no effect and what to use instead.
	Reason string `json:"reason"`
}

// Message renders the deprecation as one warning line.
func (d Deprecation) Message() string {
	location := ""
	if d.File != "" {
		location = " in " + d.File
	}
	return fmt.Sprintf("config key %s%s is deprecated and has no effect; it will be removed in codeguard %s: %s",
		d.Key, location, DeprecationRemovalVersion, d.Reason)
}

type deprecatedKeySpec struct {
	// path is the key's location; "[]" walks every element of a list.
	path   []string
	reason string
}

// deprecatedKeys lists the keys that are parsed but never read. Their struct
// fields stay in core so existing configs and SDK code keep working.
var deprecatedKeys = []deprecatedKeySpec{
	{
		path:   []string{"checks", "reliability_rules", "max_retry_attempts"},
		reason: "the retry rules judge the attempt limit and backoff they find in the code, not a configured count; remove the key",
	},
	{
		path:   []string{"checks", "data_rules", "max_unbounded_read_rows"},
		reason: "data.unbounded-read reports reads with no visible WHERE, LIMIT, cursor or streaming bound and does not compare row counts; remove the key",
	},
	{
		path:   []string{"checks", "observability_rules", "structured_logger_patterns"},
		reason: "observability.unstructured-log flags raw print/console calls with built-in patterns; remove the key, or disable the rule with checks.disabled_rules",
	},
	{
		path:   []string{"checks", "design_rules", "forbid_internal_import_cmd"},
		reason: "Go cannot import a cmd/ main package, so no design rule needs it; use design_rules.forbid_service_import_cmd and require_cmd_through_internal_cli for cmd/ layering",
	},
	{
		path:   []string{"ai", "autofix", "verify_tests"},
		reason: "fix verification always runs: `codeguard fix` and `fix-batch` run the inferred nearest tests plus ai.autofix.test_commands; remove the key",
	},
	{
		path:   []string{"rule_packs", "[]", "rules", "[]", "ai_prompt"},
		reason: "use natural_language for LLM-evaluated rules",
	},
}

// designRulesFileDeprecations lists deprecated keys inside an external
// design_rules.yml, whose top level is design_rules itself.
var designRulesFileDeprecations = []deprecatedKeySpec{
	{
		path:   []string{"forbid_internal_import_cmd"},
		reason: deprecatedKeys[3].reason,
	},
}

// DeprecationsInFile reports the deprecated keys set by the config file that
// LoadFile would read for path, including an external design rules file the
// config uses. A key counts as set when it appears in the file, whatever its
// value.
func DeprecationsInFile(path string) ([]Deprecation, error) {
	resolvedPath, err := resolveConfigPath(path)
	if err != nil {
		return nil, err
	}
	data, err := readSizeCappedFile(resolvedPath)
	if err != nil {
		return nil, err
	}
	out, err := deprecationsInData(data, deprecatedKeys, "")
	if err != nil {
		return nil, fmt.Errorf("inspect %s: %w", resolvedPath, err)
	}
	var cfg core.Config
	if err := unmarshalConfig(data, resolvedPath, &cfg); err != nil {
		return nil, err
	}
	return append(out, designRulesDeprecations(resolvedPath, cfg.Checks.DesignRulesFile)...), nil
}

// designRulesDeprecations inspects the external design rules file a config
// uses. It is best-effort: a missing or unreadable policy file is reported by
// LoadFile, so it yields no deprecations here.
func designRulesDeprecations(configPath string, explicit string) []Deprecation {
	designPath, found, err := resolveDesignRulesPath(configPath, explicit)
	if err != nil || !found {
		return nil
	}
	data, err := readSizeCappedFile(designPath)
	if err != nil {
		return nil
	}
	out, err := deprecationsInData(data, designRulesFileDeprecations, designPath)
	if err != nil {
		return nil
	}
	return out
}

// DeprecationsInData reports the deprecated keys set in a YAML or JSON
// config document.
func DeprecationsInData(data []byte) ([]Deprecation, error) {
	return deprecationsInData(data, deprecatedKeys, "")
}

func deprecationsInData(data []byte, specs []deprecatedKeySpec, file string) ([]Deprecation, error) {
	var root any
	// YAML is a superset of JSON, so one decoder serves both formats.
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	out := make([]Deprecation, 0)
	for _, spec := range specs {
		for _, key := range presentKeys(root, spec.path, "") {
			out = append(out, Deprecation{Key: key, File: file, Reason: spec.reason})
		}
	}
	return out, nil
}

// presentKeys returns the dotted paths at which path exists under node.
func presentKeys(node any, path []string, prefix string) []string {
	if len(path) == 0 {
		return []string{prefix}
	}
	segment := path[0]
	if segment == "[]" {
		list, ok := node.([]any)
		if !ok {
			return nil
		}
		out := make([]string, 0)
		for idx, item := range list {
			out = append(out, presentKeys(item, path[1:], prefix+"["+strconv.Itoa(idx)+"]")...)
		}
		return out
	}
	mapping, ok := node.(map[string]any)
	if !ok {
		return nil
	}
	child, ok := mapping[segment]
	if !ok {
		return nil
	}
	next := segment
	if prefix != "" {
		next = prefix + "." + segment
	}
	return presentKeys(child, path[1:], next)
}

// clearDeprecatedDefaults drops the defaulted values of deprecated keys so a
// config written by WriteFile (and `codeguard init -full`) does not carry
// keys that only produce deprecation warnings. User-authored rule-pack
// ai_prompt text is left in place.
func clearDeprecatedDefaults(cfg *core.Config) {
	cfg.Checks.ReliabilityRules.MaxRetryAttempts = 0
	cfg.Checks.DataRules.MaxUnboundedReadRows = 0
	cfg.Checks.ObservabilityRules.StructuredLoggerPatterns = nil
	cfg.Checks.DesignRules.ForbidInternalImportCmd = nil
	cfg.AI.AutoFix.VerifyTests = nil
}
