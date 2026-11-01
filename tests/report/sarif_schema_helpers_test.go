package report_test

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// testdata/sarif-schema-2.1.0.json is the official OASIS SARIF 2.1.0 JSON
// schema (errata01), vendored unmodified from
// https://github.com/oasis-tcs/sarif-spec/blob/main/sarif-2.1/schema/sarif-schema-2.1.0.json
// (sha256 c3b4bb2d6093897483348925aaa73af03b3e3f4bd4ca38cef26dcb4212a2682e,
// ~110 KB). It is published by the OASIS SARIF TC under the OASIS IPR policy
// (RF on RAND), which permits redistribution of the published schema. It is
// test-only input; nothing in the production build reads it.
//
// The module deliberately has no JSON Schema dependency, so this file holds a
// minimal draft-04 validator covering exactly the keywords the SARIF schema
// uses: $ref (local), type, properties, required, additionalProperties, items,
// enum, minimum, maximum, minItems, uniqueItems, pattern, anyOf, and oneOf.
// "format" and "default" are annotations and are ignored.

type sarifSchemaValidator struct {
	root map[string]any
}

func loadSARIFSchema(t *testing.T) *sarifSchemaValidator {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "sarif-schema-2.1.0.json"))
	if err != nil {
		t.Fatalf("read vendored SARIF schema: %v", err)
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		t.Fatalf("parse vendored SARIF schema: %v", err)
	}
	return &sarifSchemaValidator{root: root}
}

// validateSARIF returns every schema violation in document, each prefixed with
// its JSON path.
func (v *sarifSchemaValidator) validateSARIF(t *testing.T, document []byte) []string {
	t.Helper()
	var instance any
	if err := json.Unmarshal(document, &instance); err != nil {
		t.Fatalf("parse SARIF document: %v", err)
	}
	var errs []string
	v.validate(v.root, instance, "$", &errs)
	return errs
}

func (v *sarifSchemaValidator) resolve(ref string) map[string]any {
	node := any(v.root)
	for _, part := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
		object, ok := node.(map[string]any)
		if !ok {
			return nil
		}
		node = object[part]
	}
	resolved, _ := node.(map[string]any)
	return resolved
}

func (v *sarifSchemaValidator) validate(schema map[string]any, instance any, path string, errs *[]string) {
	if ref, ok := schema["$ref"].(string); ok {
		target := v.resolve(ref)
		if target == nil {
			*errs = append(*errs, fmt.Sprintf("%s: unresolvable $ref %s", path, ref))
			return
		}
		v.validate(target, instance, path, errs)
		return
	}
	if !checkSchemaType(schema, instance, path, errs) {
		return
	}
	checkSchemaEnum(schema, instance, path, errs)
	v.checkCombinators(schema, instance, path, errs)
	switch value := instance.(type) {
	case map[string]any:
		v.validateObject(schema, value, path, errs)
	case []any:
		v.validateArray(schema, value, path, errs)
	case string:
		if pattern, ok := schema["pattern"].(string); ok && !regexp.MustCompile(pattern).MatchString(value) {
			*errs = append(*errs, fmt.Sprintf("%s: %q does not match pattern %s", path, value, pattern))
		}
	case float64:
		if minimum, ok := schema["minimum"].(float64); ok && value < minimum {
			*errs = append(*errs, fmt.Sprintf("%s: %v is below minimum %v", path, value, minimum))
		}
		if maximum, ok := schema["maximum"].(float64); ok && value > maximum {
			*errs = append(*errs, fmt.Sprintf("%s: %v is above maximum %v", path, value, maximum))
		}
	}
}

func checkSchemaType(schema map[string]any, instance any, path string, errs *[]string) bool {
	var allowed []string
	switch declared := schema["type"].(type) {
	case string:
		allowed = []string{declared}
	case []any:
		for _, entry := range declared {
			if name, ok := entry.(string); ok {
				allowed = append(allowed, name)
			}
		}
	default:
		return true
	}
	for _, name := range allowed {
		if jsonTypeMatches(name, instance) {
			return true
		}
	}
	*errs = append(*errs, fmt.Sprintf("%s: expected type %v, got %T", path, allowed, instance))
	return false
}

func jsonTypeMatches(name string, instance any) bool {
	switch name {
	case "object":
		_, ok := instance.(map[string]any)
		return ok
	case "array":
		_, ok := instance.([]any)
		return ok
	case "string":
		_, ok := instance.(string)
		return ok
	case "boolean":
		_, ok := instance.(bool)
		return ok
	case "number":
		_, ok := instance.(float64)
		return ok
	case "integer":
		number, ok := instance.(float64)
		return ok && number == math.Trunc(number)
	case "null":
		return instance == nil
	default:
		return false
	}
}

func checkSchemaEnum(schema map[string]any, instance any, path string, errs *[]string) {
	values, ok := schema["enum"].([]any)
	if !ok {
		return
	}
	for _, candidate := range values {
		if jsonEqual(candidate, instance) {
			return
		}
	}
	*errs = append(*errs, fmt.Sprintf("%s: %v is not one of %v", path, instance, values))
}

func (v *sarifSchemaValidator) checkCombinators(schema map[string]any, instance any, path string, errs *[]string) {
	if branches, ok := schema["anyOf"].([]any); ok && v.matchingBranches(branches, instance, path) == 0 {
		*errs = append(*errs, fmt.Sprintf("%s: matches none of the anyOf alternatives", path))
	}
	if branches, ok := schema["oneOf"].([]any); ok {
		if matched := v.matchingBranches(branches, instance, path); matched != 1 {
			*errs = append(*errs, fmt.Sprintf("%s: matches %d oneOf alternatives, want exactly 1", path, matched))
		}
	}
}

func (v *sarifSchemaValidator) matchingBranches(branches []any, instance any, path string) int {
	matched := 0
	for _, branch := range branches {
		sub, ok := branch.(map[string]any)
		if !ok {
			continue
		}
		var branchErrs []string
		v.validate(sub, instance, path, &branchErrs)
		if len(branchErrs) == 0 {
			matched++
		}
	}
	return matched
}

func (v *sarifSchemaValidator) validateObject(schema map[string]any, object map[string]any, path string, errs *[]string) {
	if required, ok := schema["required"].([]any); ok {
		for _, entry := range required {
			if name, ok := entry.(string); ok {
				if _, present := object[name]; !present {
					*errs = append(*errs, fmt.Sprintf("%s: missing required property %q", path, name))
				}
			}
		}
	}
	properties, _ := schema["properties"].(map[string]any)
	for name, value := range object {
		childPath := path + "." + name
		if propertySchema, ok := properties[name].(map[string]any); ok {
			v.validate(propertySchema, value, childPath, errs)
			continue
		}
		switch additional := schema["additionalProperties"].(type) {
		case bool:
			if !additional {
				*errs = append(*errs, fmt.Sprintf("%s: property not allowed by the schema", childPath))
			}
		case map[string]any:
			v.validate(additional, value, childPath, errs)
		}
	}
}

func (v *sarifSchemaValidator) validateArray(schema map[string]any, array []any, path string, errs *[]string) {
	if minItems, ok := schema["minItems"].(float64); ok && float64(len(array)) < minItems {
		*errs = append(*errs, fmt.Sprintf("%s: has %d items, want at least %v", path, len(array), minItems))
	}
	if unique, ok := schema["uniqueItems"].(bool); ok && unique {
		seen := map[string]bool{}
		for i, item := range array {
			encoded, _ := json.Marshal(item)
			if seen[string(encoded)] {
				*errs = append(*errs, fmt.Sprintf("%s[%d]: duplicate item in a uniqueItems array", path, i))
			}
			seen[string(encoded)] = true
		}
	}
	if items, ok := schema["items"].(map[string]any); ok {
		for i, item := range array {
			v.validate(items, item, fmt.Sprintf("%s[%d]", path, i), errs)
		}
	}
}

func jsonEqual(a any, b any) bool {
	left, _ := json.Marshal(a)
	right, _ := json.Marshal(b)
	return string(left) == string(right)
}
