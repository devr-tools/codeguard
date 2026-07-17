package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"gopkg.in/yaml.v3"
)

var defaultDesignRulesNames = []string{"design_rules.yml", "design_rules.yaml"}

// designRulesOverlay retains every field explicitly present in the external
// policy or inline overrides so values such as zero and an empty slice can be
// restored after profile defaults are applied. It is inactive when no external
// policy was loaded, preserving the historical defaulting behavior of
// inline-only configurations.
type designRulesOverlay struct {
	active bool
	rules  core.DesignRulesConfig
	fields map[string]struct{}
}

func (o designRulesOverlay) apply(dst *core.DesignRulesConfig) {
	if !o.active {
		return
	}
	copyDesignRuleFields(dst, o.rules, o.fields)
}

func loadExternalDesignRules(cfg *core.Config, mainData []byte, configPath string) (designRulesOverlay, error) {
	inlineFields, err := inlineDesignRuleFields(mainData)
	if err != nil {
		return designRulesOverlay{}, fmt.Errorf("inspect inline design_rules: %w", err)
	}
	inlineRules := cfg.Checks.DesignRules

	policyPath, found, err := resolveDesignRulesPath(configPath, cfg.Checks.DesignRulesFile)
	if err != nil {
		return designRulesOverlay{}, err
	}
	if !found {
		return designRulesOverlay{}, nil
	}

	data, err := readSizeCappedFile(policyPath)
	if err != nil {
		return designRulesOverlay{}, fmt.Errorf("read design rules file %q: %w", policyPath, err)
	}
	var external core.DesignRulesConfig
	if err := unmarshalDesignRules(data, policyPath, &external); err != nil {
		return designRulesOverlay{}, fmt.Errorf("parse design rules file %q: %w", policyPath, err)
	}
	externalFields, err := topLevelDesignRuleFields(data)
	if err != nil {
		return designRulesOverlay{}, fmt.Errorf("inspect design rules file %q: %w", policyPath, err)
	}

	cfg.Checks.DesignRules = external
	copyDesignRuleFields(&cfg.Checks.DesignRules, inlineRules, inlineFields)
	for name := range inlineFields {
		externalFields[name] = struct{}{}
	}
	return designRulesOverlay{active: true, rules: cfg.Checks.DesignRules, fields: externalFields}, nil
}

func readSizeCappedFile(path string) ([]byte, error) {
	f, err := os.Open(path) //nolint:gosec // path is contained beneath the project root above
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	data, err := io.ReadAll(io.LimitReader(f, maxConfigFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxConfigFileBytes {
		return nil, fmt.Errorf("file exceeds the %d-byte config size limit", maxConfigFileBytes)
	}
	return data, nil
}

func unmarshalDesignRules(data []byte, path string, rules *core.DesignRulesConfig) error {
	if strings.EqualFold(filepath.Ext(path), ".json") {
		return json.Unmarshal(data, rules)
	}
	return yaml.Unmarshal(data, rules)
}

func inlineDesignRuleFields(data []byte) (map[string]struct{}, error) {
	var document struct {
		Checks struct {
			DesignRules map[string]yaml.Node `yaml:"design_rules"`
		} `yaml:"checks"`
	}
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	fields := make(map[string]struct{}, len(document.Checks.DesignRules))
	for name := range document.Checks.DesignRules {
		fields[name] = struct{}{}
	}
	return fields, nil
}

func topLevelDesignRuleFields(data []byte) (map[string]struct{}, error) {
	var document map[string]yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	fields := make(map[string]struct{}, len(document))
	for name := range document {
		fields[name] = struct{}{}
	}
	return fields, nil
}

func copyDesignRuleFields(dst *core.DesignRulesConfig, src core.DesignRulesConfig, fields map[string]struct{}) {
	dstValue := reflect.ValueOf(dst).Elem()
	srcValue := reflect.ValueOf(src)
	typ := dstValue.Type()
	for i := 0; i < typ.NumField(); i++ {
		name := strings.Split(typ.Field(i).Tag.Get("yaml"), ",")[0]
		if _, ok := fields[name]; ok {
			dstValue.Field(i).Set(srcValue.Field(i))
		}
	}
}
