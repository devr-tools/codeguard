package mcp_test

import (
	"encoding/json"
	"fmt"
	"math"
)

// validateSchema checks value against the JSON Schema subset codeguard's tool
// output schemas use: type, properties, required, items, enum, oneOf and
// anyOf. It returns nil when value is valid. Unknown keywords are ignored, as
// a permissive validator would.
func validateSchema(schema map[string]any, value any, at string) error {
	if typ, ok := schema["type"].(string); ok {
		if err := validateType(typ, value, at); err != nil {
			return err
		}
	}
	if enum, ok := schema["enum"].([]any); ok && !enumContains(enum, value) {
		return fmt.Errorf("%s: %v not in enum %v", at, value, enum)
	}
	if err := validateObjectKeywords(schema, value, at); err != nil {
		return err
	}
	if items, ok := schema["items"].(map[string]any); ok {
		if list, isList := value.([]any); isList {
			for i, item := range list {
				if err := validateSchema(items, item, fmt.Sprintf("%s[%d]", at, i)); err != nil {
					return err
				}
			}
		}
	}
	if branches, ok := schema["oneOf"].([]any); ok {
		if matched := countMatchingBranches(branches, value, at); matched != 1 {
			return fmt.Errorf("%s: matches %d oneOf branches, want exactly 1", at, matched)
		}
	}
	if branches, ok := schema["anyOf"].([]any); ok && countMatchingBranches(branches, value, at) == 0 {
		return fmt.Errorf("%s: matches no anyOf branch", at)
	}
	return nil
}

func validateObjectKeywords(schema map[string]any, value any, at string) error {
	object, isObject := value.(map[string]any)
	if !isObject {
		return nil
	}
	if required, ok := schema["required"].([]any); ok {
		for _, key := range required {
			if _, present := object[fmt.Sprint(key)]; !present {
				return fmt.Errorf("%s: missing required %q", at, key)
			}
		}
	}
	props, _ := schema["properties"].(map[string]any)
	for key, sub := range props {
		subSchema, ok := sub.(map[string]any)
		child, present := object[key]
		if !ok || !present {
			continue
		}
		if err := validateSchema(subSchema, child, at+"."+key); err != nil {
			return err
		}
	}
	return nil
}

func countMatchingBranches(branches []any, value any, at string) int {
	matched := 0
	for _, branch := range branches {
		if schema, ok := branch.(map[string]any); ok && validateSchema(schema, value, at) == nil {
			matched++
		}
	}
	return matched
}

func validateType(typ string, value any, at string) error {
	ok := false
	switch typ {
	case "object":
		_, ok = value.(map[string]any)
	case "array":
		_, ok = value.([]any)
	case "string":
		_, ok = value.(string)
	case "boolean":
		_, ok = value.(bool)
	case "number":
		_, ok = value.(float64)
	case "integer":
		number, isNumber := value.(float64)
		ok = isNumber && number == math.Trunc(number)
	case "null":
		ok = value == nil
	default:
		ok = true
	}
	if !ok {
		return fmt.Errorf("%s: %T is not %s", at, value, typ)
	}
	return nil
}

func enumContains(enum []any, value any) bool {
	for _, candidate := range enum {
		if candidate == value {
			return true
		}
	}
	return false
}

// decodeSchema decodes a raw JSON schema into the generic form validateSchema
// walks.
func decodeSchema(raw []byte) (map[string]any, error) {
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil, err
	}
	return schema, nil
}
