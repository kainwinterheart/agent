// =========================
// SCHEMA UTILS
// =========================
package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/qri-io/jsonschema"
)

func pad(line string, level int) string {
	spaces := level * 2
	return strings.Repeat(" ", spaces) + line
}

func SchemaToExample(schema map[string]interface{}) string {
	return buildValue(schema, 0)
}

func buildValue(node map[string]interface{}, level int) string {
	nodeType, _ := node["type"].(string)

	// Handle enum arrays - check both []interface{} (from JSON) and []string (from Go code)
	if enums, ok := node["enum"].([]interface{}); ok && len(enums) > 0 {
		return buildEnum(enums, node)
	}
	if enumsStr, ok := node["enum"].([]string); ok && len(enumsStr) > 0 {
		enums := make([]interface{}, len(enumsStr))
		for i, e := range enumsStr {
			enums[i] = e
		}
		return buildEnum(enums, node)
	}

	switch nodeType {
	case "string":
		desc, exists := node["description"].(string)
		if !exists {
			desc = "example string"
		}
		return MarshalJSON(desc)
	case "boolean":
		desc, exists := node["description"].(string)
		if !exists {
			desc = "true/false"
		}
		return desc
	case "integer":
		desc, exists := node["description"].(string)
		if !exists {
			desc = "0"
		}
		return desc
	case "number":
		desc, exists := node["description"].(string)
		if !exists {
			desc = "0.0"
		}
		return desc
	case "array":
		items, _ := node["items"].(map[string]interface{})
		// Do NOT increment level for array items - items inherit the array's level
		return "[" + buildValue(items, level) + "]"
	case "object":
		props, _ := node["properties"].(map[string]interface{})
		var sb strings.Builder
		sb.WriteString("{\n")
		// Use required array order for deterministic output
		requiredKeys := []string{}
		if req, ok := node["required"].([]interface{}); ok {
			for _, rk := range req {
				if s, ok := rk.(string); ok {
					requiredKeys = append(requiredKeys, s)
				}
			}
		} else if reqStr, ok := node["required"].([]string); ok {
			requiredKeys = reqStr
		}
		keys := requiredKeys
		if len(keys) == 0 {
			for k := range props {
				keys = append(keys, k)
			}
		}
		for _, key := range keys {
			prop := props[key]
			propMap, _ := prop.(map[string]interface{})
			// Property key at level+1, property VALUE at level+2 for nested structures
			line := pad(
				MarshalJSON(key)+": "+buildValue(propMap, level+2)+",",
				level+1,
			)
			sb.WriteString(line + "\n")
		}
		// Remove trailing comma from last property line
		lastLineIdx := strings.LastIndex(sb.String(), "\n")
		if lastLineIdx > 0 {
			content := sb.String()[:lastLineIdx]
			lines := strings.Split(content, "\n")
			if len(lines) > 0 {
				lines[len(lines)-1] = strings.TrimSuffix(lines[len(lines)-1], ",")
			}
			sb.Reset()
			sb.WriteString(strings.Join(lines, "\n"))
			sb.WriteString("\n")
		}
		sb.WriteString(pad("}", level))
		return sb.String()
	default:
		return "example"
	}
}

func buildEnum(enums []interface{}, node map[string]interface{}) string {
	// If a non-empty description exists, use it as the example
	if desc, ok := node["description"].(string); ok && desc != "" {
		return desc
	}
	strs := make([]string, len(enums))
	for i, e := range enums {
		strs[i] = MarshalJSON(e.(string))
	}
	return strings.Join(strs, "/")
}

// CompileSchema compiles a schema from a map[string]interface{} into a *jsonschema.Schema.
func CompileSchema(schema map[string]interface{}) *jsonschema.Schema {
	data := []byte(MarshalJSON(schema))
	s := &jsonschema.Schema{}
	if err := s.UnmarshalJSON(data); err != nil {
		panic(err)
	}
	return s
}

// ValidateSchema validates data (as a map) against the given schema.
// Returns a descriptive error if validation fails, nil otherwise.
func ValidateSchema(data map[string]interface{}, schema map[string]interface{}) error {
	s := CompileSchema(schema)
	vs := s.Validate(context.Background(), data)
	if vs.IsValid() {
		return nil
	}
	// Build a combined error message from all validation errors
	var parts []string
	if vs.Errs != nil {
		for _, ke := range *vs.Errs {
			if ke.PropertyPath != "" {
				parts = append(parts, fmt.Sprintf("Error within %s: %s", ke.PropertyPath, ke.Message))
			} else {
				parts = append(parts, ke.Message)
			}
		}
	}
	if len(parts) == 0 {
		return fmt.Errorf("validation failed")
	}
	return fmt.Errorf("%s", strings.Join(parts, "; "))
}

func mergeMaps(base, extra map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range base {
		result[k] = v
	}
	for k, v := range extra {
		result[k] = v
	}
	return result
}
