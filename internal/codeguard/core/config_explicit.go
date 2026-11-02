package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// explicitKeys records which tracked int keys of a config struct were stated
// explicitly, one bit per key in the struct's explicitIntFields order. It is a
// plain value, so every struct copy, merge by assignment, or clone carries it
// without aliasing.
type explicitKeys uint32

// explicitIntField ties a tracked config key to its Go field name and value.
type explicitIntField struct {
	key  string
	name string
	ptr  *int
}

// explicitIntConfig is implemented by the config structs whose int keys can
// carry an explicit 0. The helpers below share all bookkeeping between them.
type explicitIntConfig interface {
	explicitIntFields() []explicitIntField
	explicitMask() *explicitKeys
}

func explicitIndex(fields []explicitIntField, key string) int {
	key = strings.TrimSpace(key)
	for idx, field := range fields {
		if strings.EqualFold(field.key, key) || strings.EqualFold(field.name, key) {
			return idx
		}
	}
	return -1
}

func explicitKeyNames(fields []explicitIntField) []string {
	keys := make([]string, len(fields))
	for idx, field := range fields {
		keys[idx] = field.key
	}
	return keys
}

// updateExplicit sets or clears the explicit bit for each key. It checks every
// key before changing anything, so an unknown key leaves the mask untouched.
func updateExplicit(cfg explicitIntConfig, set bool, keys []string) error {
	fields := cfg.explicitIntFields()
	var bits explicitKeys
	for _, key := range keys {
		idx := explicitIndex(fields, key)
		if idx < 0 {
			return fmt.Errorf("config key %q does not support an explicit 0 (supported: %s)", key, strings.Join(explicitKeyNames(fields), ", "))
		}
		bits |= 1 << idx
	}
	mask := cfg.explicitMask()
	if set {
		*mask |= bits
	} else {
		*mask &^= bits
	}
	return nil
}

func isExplicit(cfg explicitIntConfig, key string) bool {
	idx := explicitIndex(cfg.explicitIntFields(), key)
	return idx >= 0 && *cfg.explicitMask()&(1<<idx) != 0
}

// explicitZeroKeys lists the tracked keys that are explicit and currently 0:
// the keys an encoder must write even though their tags say omitempty.
func explicitZeroKeys(cfg explicitIntConfig) []string {
	mask := *cfg.explicitMask()
	var keys []string
	for idx, field := range cfg.explicitIntFields() {
		if mask&(1<<idx) != 0 && *field.ptr == 0 {
			keys = append(keys, field.key)
		}
	}
	return keys
}

// markYAMLPresence sets the explicit bit for every tracked key that appears
// with a non-null value in a decoded YAML mapping, following aliases and
// `<<` merge keys the way the decoder does.
func markYAMLPresence(cfg explicitIntConfig, node *yaml.Node) {
	for node != nil && node.Kind == yaml.AliasNode {
		node = node.Alias
	}
	if node == nil {
		return
	}
	switch node.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, child := range node.Content {
			markYAMLPresence(cfg, child)
		}
	case yaml.MappingNode:
		fields := cfg.explicitIntFields()
		mask := cfg.explicitMask()
		for idx := 0; idx+1 < len(node.Content); idx += 2 {
			key, value := node.Content[idx], node.Content[idx+1]
			if key.Value == "<<" && key.ShortTag() == "!!merge" {
				markYAMLPresence(cfg, value)
				continue
			}
			if value.Kind == yaml.ScalarNode && value.ShortTag() == "!!null" {
				continue
			}
			for fieldIdx, field := range fields {
				if key.Value == field.key {
					*mask |= 1 << fieldIdx
				}
			}
		}
	}
}

// markJSONPresence sets the explicit bit for every tracked key present with a
// non-null value in a JSON object. encoding/json matches object keys to tags
// case-insensitively, so presence does too.
func markJSONPresence(cfg explicitIntConfig, data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	fields := cfg.explicitIntFields()
	mask := cfg.explicitMask()
	for key, value := range raw {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			continue
		}
		for fieldIdx, field := range fields {
			if strings.EqualFold(key, field.key) {
				*mask |= 1 << fieldIdx
			}
		}
	}
	return nil
}

// appendYAMLExplicitZeros adds `key: 0` for explicit zero keys that the
// omitempty tags dropped from an encoded mapping.
func appendYAMLExplicitZeros(node *yaml.Node, keys []string) {
	for _, key := range keys {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: "0"},
		)
	}
}

// appendJSONExplicitZeros adds `"key":0` for explicit zero keys that the
// omitempty tags dropped from an encoded object.
func appendJSONExplicitZeros(data []byte, keys []string) []byte {
	if len(keys) == 0 {
		return data
	}
	trimmed := bytes.TrimRight(data, " \n\t\r")
	out := append([]byte(nil), trimmed[:len(trimmed)-1]...)
	needComma := len(bytes.TrimSpace(out)) > 1
	for _, key := range keys {
		if needComma {
			out = append(out, ',')
		}
		encodedKey, _ := json.Marshal(key) //nolint:errchkjson // a Go string always encodes
		out = append(out, encodedKey...)
		out = append(out, ":0"...)
		needComma = true
	}
	return append(out, '}')
}
