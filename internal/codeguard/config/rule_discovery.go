package config

import (
	"reflect"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	rulespkg "github.com/devr-tools/codeguard/internal/codeguard/rules"
)

var configType = reflect.TypeOf(core.Config{})

// KnownConfigKey reports whether key is a dotted path of yaml field names
// that exists in the config structs, for example
// checks.quality_rules.max_parameters. It walks types, not values, so a key
// under an unset pointer is still known.
func KnownConfigKey(key string) bool {
	current := configType
	for _, segment := range strings.Split(strings.TrimSpace(key), ".") {
		current = derefType(current)
		if current.Kind() != reflect.Struct {
			return false
		}
		field, ok := fieldByYAMLName(current, segment)
		if !ok {
			return false
		}
		current = field.Type
	}
	return true
}

// LookupConfigKey returns the value at a dotted yaml path in cfg. Pointers are
// dereferenced; an unset pointer yields (nil, true) when the key itself is
// known, so callers can tell "unset" apart from "no such key".
func LookupConfigKey(cfg core.Config, key string) (any, bool) {
	if !KnownConfigKey(key) {
		return nil, false
	}
	current := reflect.ValueOf(cfg)
	for _, segment := range strings.Split(strings.TrimSpace(key), ".") {
		for current.Kind() == reflect.Pointer {
			if current.IsNil() {
				return nil, true
			}
			current = current.Elem()
		}
		field, _ := fieldByYAMLName(current.Type(), segment)
		current = current.FieldByIndex(field.Index)
	}
	if current.Kind() == reflect.Pointer {
		if current.IsNil() {
			return nil, true
		}
		current = current.Elem()
	}
	return current.Interface(), true
}

// enableKeysOnWhenUnset are the enable keys whose unset (nil) value means on.
// checks.context runs in full scans unless set; the other section toggles
// are off when unset. Rule-level switches (taint_go, local_precision, ...)
// treat nil as on, matching how their checks read them.
var enableKeysOnWhenUnset = map[string]bool{
	"checks.context": true,
}

// RuleEnabled reports whether meta would run under cfg in a full scan: every
// enable key it lists resolves to true. cfg should already have defaults
// applied (LoadFile and the profile helpers do this).
func RuleEnabled(cfg core.Config, meta core.RuleMetadata) bool {
	for _, key := range meta.ConfigKeys {
		if key.Role != core.RuleConfigKeyRoleEnable {
			continue
		}
		if !enableKeyOn(cfg, key.Key) {
			return false
		}
	}
	return true
}

func enableKeyOn(cfg core.Config, key string) bool {
	value, ok := LookupConfigKey(cfg, key)
	if !ok {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case nil:
		if strings.HasPrefix(key, "checks.") && strings.Count(key, ".") == 1 {
			return enableKeysOnWhenUnset[key]
		}
		return true
	default:
		return false
	}
}

// RuleProfileNames lists the profiles under which meta runs, starting from
// the recommended sections (what `codeguard init` and zero-config scans turn
// on). "default" stands for no profile.
func RuleProfileNames(meta core.RuleMetadata) []string {
	profiles := ProfileList()
	names := make([]string, 0, len(profiles)+1)
	names = append(names, "")
	for _, profile := range profiles {
		names = append(names, profile.Name)
	}
	out := make([]string, 0, len(names))
	for _, name := range names {
		cfg := core.Config{Profile: name, Checks: core.CheckConfig{UseRecommendedDefaults: true}}
		ApplyDefaults(&cfg)
		if !RuleEnabled(cfg, meta) {
			continue
		}
		if name == "" {
			name = "default"
		}
		out = append(out, name)
	}
	return out
}

// RuleSectionKey returns the section id (quality, security, supply_chain, ...)
// a rule reports under.
func RuleSectionKey(meta core.RuleMetadata) string {
	return rulespkg.SectionKey(meta)
}

func derefType(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}

func fieldByYAMLName(t reflect.Type, name string) (reflect.StructField, bool) {
	for idx := 0; idx < t.NumField(); idx++ {
		field := t.Field(idx)
		if field.PkgPath != "" {
			continue
		}
		if tagName := strings.Split(field.Tag.Get("yaml"), ",")[0]; tagName == name && tagName != "" && tagName != "-" {
			return field, true
		}
	}
	return reflect.StructField{}, false
}
