package cli

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

type configOverrideValues []string

func (v *configOverrideValues) String() string {
	if v == nil {
		return ""
	}
	return strings.Join(*v, ",")
}

func (v *configOverrideValues) Set(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("override must use key=value")
	}
	*v = append(*v, trimmed)
	return nil
}

func applyConfigOverrides(cfg *service.Config, overrides []string) error {
	for _, override := range overrides {
		key, value, ok := strings.Cut(override, "=")
		if !ok || strings.TrimSpace(key) == "" {
			return fmt.Errorf("override %q must use key=value", override)
		}
		if err := setConfigOverride(reflect.ValueOf(cfg).Elem(), strings.TrimSpace(key), strings.TrimSpace(value)); err != nil {
			return err
		}
	}
	if len(overrides) > 0 {
		service.ApplyDefaults(cfg)
		if err := service.ValidateConfig(*cfg); err != nil {
			return err
		}
	}
	return nil
}

type overrideSegment struct {
	name  string
	index *int
}

func setConfigOverride(root reflect.Value, path string, raw string) error {
	segments, err := parseOverridePath(path)
	if err != nil {
		return err
	}
	current := root
	for idx, segment := range segments {
		last := idx == len(segments)-1
		next, err := resolveOverrideSegment(current, segment, path)
		if err != nil {
			return err
		}
		if last {
			if err := assignOverrideValue(next, raw, path); err != nil {
				return err
			}
			return nil
		}
		current = dereferenceOverrideValue(next)
		if current.Kind() != reflect.Struct && current.Kind() != reflect.Slice {
			return fmt.Errorf("override %q cannot descend through %s", path, segment.name)
		}
	}
	return nil
}

func parseOverridePath(path string) ([]overrideSegment, error) {
	parts := strings.Split(strings.TrimSpace(path), ".")
	segments := make([]overrideSegment, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			return nil, fmt.Errorf("override path %q contains an empty segment", path)
		}
		segment := overrideSegment{name: part}
		if open := strings.Index(part, "["); open >= 0 {
			if !strings.HasSuffix(part, "]") {
				return nil, fmt.Errorf("override path %q has malformed index", path)
			}
			segment.name = part[:open]
			indexText := part[open+1 : len(part)-1]
			index, err := strconv.Atoi(indexText)
			if err != nil || index < 0 {
				return nil, fmt.Errorf("override path %q has invalid index %q", path, indexText)
			}
			segment.index = &index
		}
		if strings.TrimSpace(segment.name) == "" {
			return nil, fmt.Errorf("override path %q contains an empty segment", path)
		}
		segments = append(segments, segment)
	}
	return segments, nil
}

func resolveOverrideSegment(value reflect.Value, segment overrideSegment, fullPath string) (reflect.Value, error) {
	value = dereferenceOverrideValue(value)
	if value.Kind() != reflect.Struct {
		return reflect.Value{}, fmt.Errorf("override %q cannot select %q from %s", fullPath, segment.name, value.Kind())
	}
	field, ok := overrideFieldByConfigName(value, segment.name)
	if !ok {
		return reflect.Value{}, fmt.Errorf("override %q references unknown config field %q", fullPath, segment.name)
	}
	if segment.index == nil {
		return field, nil
	}
	field = dereferenceOverrideValue(field)
	if field.Kind() != reflect.Slice {
		return reflect.Value{}, fmt.Errorf("override %q indexes non-list field %q", fullPath, segment.name)
	}
	index := *segment.index
	if index > field.Len() {
		return reflect.Value{}, fmt.Errorf("override %q index %d is beyond %q length %d", fullPath, index, segment.name, field.Len())
	}
	if index == field.Len() {
		field.Set(reflect.Append(field, reflect.Zero(field.Type().Elem())))
	}
	return field.Index(index), nil
}

func overrideFieldByConfigName(value reflect.Value, name string) (reflect.Value, bool) {
	valueType := value.Type()
	for idx := 0; idx < value.NumField(); idx++ {
		fieldInfo := valueType.Field(idx)
		if fieldInfo.PkgPath != "" {
			continue
		}
		if overrideFieldNameMatches(fieldInfo, name) {
			return value.Field(idx), true
		}
	}
	return reflect.Value{}, false
}

func overrideFieldNameMatches(field reflect.StructField, name string) bool {
	for _, tagName := range []string{field.Tag.Get("json"), field.Tag.Get("yaml")} {
		tagName = strings.Split(tagName, ",")[0]
		if tagName == name {
			return true
		}
	}
	return strings.EqualFold(field.Name, name)
}

func dereferenceOverrideValue(value reflect.Value) reflect.Value {
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			value.Set(reflect.New(value.Type().Elem()))
		}
		value = value.Elem()
	}
	return value
}

func assignOverrideValue(value reflect.Value, raw string, path string) error {
	if !value.CanSet() {
		return fmt.Errorf("override %q is not settable", path)
	}
	if value.Kind() == reflect.Pointer {
		if strings.EqualFold(raw, "null") {
			value.Set(reflect.Zero(value.Type()))
			return nil
		}
		if value.IsNil() {
			value.Set(reflect.New(value.Type().Elem()))
		}
		return assignOverrideValue(value.Elem(), raw, path)
	}
	switch value.Kind() {
	case reflect.String:
		value.SetString(raw)
		return nil
	case reflect.Bool:
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			return fmt.Errorf("override %q expects bool, got %q", path, raw)
		}
		value.SetBool(parsed)
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		parsed, err := strconv.ParseInt(raw, 10, value.Type().Bits())
		if err != nil {
			return fmt.Errorf("override %q expects integer, got %q", path, raw)
		}
		value.SetInt(parsed)
		return nil
	case reflect.Slice:
		if value.Type().Elem().Kind() != reflect.String {
			return fmt.Errorf("override %q only supports complete assignment for string lists", path)
		}
		value.Set(reflect.ValueOf(parseStringListOverride(raw)))
		return nil
	default:
		return fmt.Errorf("override %q cannot assign %s", path, value.Kind())
	}
}

func parseStringListOverride(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	if strings.HasPrefix(raw, "[") {
		var values []string
		if err := json.Unmarshal([]byte(raw), &values); err == nil {
			return values
		}
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}
