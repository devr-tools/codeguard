package contracts

import (
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// isOpenAPIFile matches the conventional OpenAPI/Swagger document names:
// openapi.{yaml,yml,json} and swagger.{yaml,yml,json} (including dotted
// variants such as swagger.v1.json).
func isOpenAPIFile(rel string) bool {
	base := strings.ToLower(filepath.Base(filepath.ToSlash(rel)))
	ext := filepath.Ext(base)
	switch ext {
	case ".yaml", ".yml", ".json":
	default:
		return false
	}
	name := strings.TrimSuffix(base, ext)
	return name == "openapi" || name == "swagger" ||
		strings.HasPrefix(name, "openapi.") || strings.HasPrefix(name, "swagger.")
}

// parseOpenAPIDoc unmarshals a YAML or JSON document (JSON is a YAML subset).
func parseOpenAPIDoc(data []byte) map[string]any {
	if len(data) == 0 {
		return nil
	}
	doc := map[string]any{}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil
	}
	return doc
}

func asMap(value any) map[string]any {
	m, _ := value.(map[string]any)
	return m
}

func mapValue(m map[string]any, key string) map[string]any {
	if m == nil {
		return nil
	}
	return asMap(m[key])
}

func listValue(m map[string]any, key string) []any {
	if m == nil {
		return nil
	}
	list, _ := m[key].([]any)
	return list
}

func asBool(value any) bool {
	b, _ := value.(bool)
	return b
}

func paramKey(param map[string]any) string {
	return fmt.Sprintf("%v|%v", param["name"], param["in"])
}

func requiredParamSet(op map[string]any) map[string]bool {
	out := map[string]bool{}
	for _, raw := range listValue(op, "parameters") {
		param := asMap(raw)
		if param == nil || !asBool(param["required"]) {
			continue
		}
		out[paramKey(param)] = true
	}
	return out
}

// requiredBodyFields maps request body content types to their sets of
// required schema fields.
func requiredBodyFields(op map[string]any) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	content := mapValue(mapValue(op, "requestBody"), "content")
	for contentType, raw := range content {
		schema := mapValue(asMap(raw), "schema")
		fields := map[string]bool{}
		for _, field := range listValue(schema, "required") {
			fields[fmt.Sprintf("%v", field)] = true
		}
		out[contentType] = fields
	}
	return out
}
