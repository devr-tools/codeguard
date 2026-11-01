package core

import (
	"encoding/json"

	"gopkg.in/yaml.v3"
)

// riskScoringAlias and productionRiskAlias drop the methods below so the
// codecs can decode and encode the plain fields without recursing.
type (
	riskScoringAlias    RiskScoringConfig
	productionRiskAlias ProductionRiskConfig
)

func (c *RiskScoringConfig) explicitIntFields() []explicitIntField {
	return []explicitIntField{
		{"changed_file_weight", "ChangedFileWeight", &c.ChangedFileWeight},
		{"fail_finding_weight", "FailFindingWeight", &c.FailFindingWeight},
		{"warn_finding_weight", "WarnFindingWeight", &c.WarnFindingWeight},
		{"security_weight", "SecurityWeight", &c.SecurityWeight},
		{"supply_chain_weight", "SupplyChainWeight", &c.SupplyChainWeight},
		{"coverage_gap_weight", "CoverageGapWeight", &c.CoverageGapWeight},
		{"ai_provenance_weight", "AIProvenanceWeight", &c.AIProvenanceWeight},
		{"ai_signal_weight", "AISignalWeight", &c.AISignalWeight},
		{"slop_score_divisor", "SlopScoreDivisor", &c.SlopScoreDivisor},
	}
}

func (c *RiskScoringConfig) explicitMask() *explicitKeys { return &c.explicit }

// SetExplicit marks keys as explicitly set, so a 0 in those fields survives
// ApplyDefaults instead of being replaced by the default. Keys are the YAML
// names ("security_weight") or the Go field names ("SecurityWeight"). It
// returns an error, and marks nothing, if any key is not one of the weights or
// the divisor.
func (c *RiskScoringConfig) SetExplicit(keys ...string) error {
	return updateExplicit(c, true, keys)
}

// ClearExplicit undoes SetExplicit (or an explicit key read from a config
// file), so a 0 in those fields takes the default again.
func (c *RiskScoringConfig) ClearExplicit(keys ...string) error {
	return updateExplicit(c, false, keys)
}

// IsExplicit reports whether key was marked explicit by SetExplicit, a config
// file, or a -set override.
func (c *RiskScoringConfig) IsExplicit(key string) bool { return isExplicit(c, key) }

// ExplicitKeys lists the keys SetExplicit accepts.
func (c *RiskScoringConfig) ExplicitKeys() []string {
	return explicitKeyNames(c.explicitIntFields())
}

// IsZero reports whether the config is entirely unset, explicit marks
// included, so an omitempty parent does not drop explicit zeros on encode.
func (c RiskScoringConfig) IsZero() bool {
	return c == RiskScoringConfig{}
}

// UnmarshalYAML decodes the fields and records which weights were present.
func (c *RiskScoringConfig) UnmarshalYAML(node *yaml.Node) error {
	alias := riskScoringAlias(*c)
	if err := node.Decode(&alias); err != nil {
		return err
	}
	*c = RiskScoringConfig(alias)
	markYAMLPresence(c, node)
	return nil
}

// UnmarshalJSON decodes the fields and records which weights were present.
func (c *RiskScoringConfig) UnmarshalJSON(data []byte) error {
	alias := riskScoringAlias(*c)
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*c = RiskScoringConfig(alias)
	return markJSONPresence(c, data)
}

// MarshalYAML writes explicit zero weights that omitempty would drop.
func (c RiskScoringConfig) MarshalYAML() (interface{}, error) {
	var node yaml.Node
	if err := node.Encode(riskScoringAlias(c)); err != nil {
		return nil, err
	}
	appendYAMLExplicitZeros(&node, explicitZeroKeys(&c))
	return &node, nil
}

// MarshalJSON writes explicit zero weights that omitempty would drop.
func (c RiskScoringConfig) MarshalJSON() ([]byte, error) {
	data, err := json.Marshal(riskScoringAlias(c))
	if err != nil {
		return nil, err
	}
	return appendJSONExplicitZeros(data, explicitZeroKeys(&c)), nil
}

func (c *ProductionRiskConfig) explicitIntFields() []explicitIntField {
	return []explicitIntField{
		{"reliability_weight", "ReliabilityWeight", &c.ReliabilityWeight},
		{"data_weight", "DataWeight", &c.DataWeight},
		{"fail_weight", "FailWeight", &c.FailWeight},
		{"warn_weight", "WarnWeight", &c.WarnWeight},
	}
}

func (c *ProductionRiskConfig) explicitMask() *explicitKeys { return &c.explicit }

// SetExplicit marks keys as explicitly set, so a 0 in those fields survives
// ApplyDefaults instead of being replaced by the default. Keys are the YAML
// names ("fail_weight") or the Go field names ("FailWeight"). It returns an
// error, and marks nothing, if any key is not one of the four weights.
func (c *ProductionRiskConfig) SetExplicit(keys ...string) error {
	return updateExplicit(c, true, keys)
}

// ClearExplicit undoes SetExplicit (or an explicit key read from a config
// file), so a 0 in those fields takes the default again.
func (c *ProductionRiskConfig) ClearExplicit(keys ...string) error {
	return updateExplicit(c, false, keys)
}

// IsExplicit reports whether key was marked explicit by SetExplicit, a config
// file, or a -set override.
func (c *ProductionRiskConfig) IsExplicit(key string) bool { return isExplicit(c, key) }

// ExplicitKeys lists the keys SetExplicit accepts.
func (c *ProductionRiskConfig) ExplicitKeys() []string {
	return explicitKeyNames(c.explicitIntFields())
}

// IsZero reports whether the config is entirely unset, explicit marks
// included, so an omitempty parent does not drop explicit zeros on encode.
func (c ProductionRiskConfig) IsZero() bool {
	return c == ProductionRiskConfig{}
}

// UnmarshalYAML decodes the fields and records which weights were present.
func (c *ProductionRiskConfig) UnmarshalYAML(node *yaml.Node) error {
	alias := productionRiskAlias(*c)
	if err := node.Decode(&alias); err != nil {
		return err
	}
	*c = ProductionRiskConfig(alias)
	markYAMLPresence(c, node)
	return nil
}

// UnmarshalJSON decodes the fields and records which weights were present.
func (c *ProductionRiskConfig) UnmarshalJSON(data []byte) error {
	alias := productionRiskAlias(*c)
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*c = ProductionRiskConfig(alias)
	return markJSONPresence(c, data)
}

// MarshalYAML writes explicit zero weights that omitempty would drop.
func (c ProductionRiskConfig) MarshalYAML() (interface{}, error) {
	var node yaml.Node
	if err := node.Encode(productionRiskAlias(c)); err != nil {
		return nil, err
	}
	appendYAMLExplicitZeros(&node, explicitZeroKeys(&c))
	return &node, nil
}

// MarshalJSON writes explicit zero weights that omitempty would drop.
func (c ProductionRiskConfig) MarshalJSON() ([]byte, error) {
	data, err := json.Marshal(productionRiskAlias(c))
	if err != nil {
		return nil, err
	}
	return appendJSONExplicitZeros(data, explicitZeroKeys(&c)), nil
}
