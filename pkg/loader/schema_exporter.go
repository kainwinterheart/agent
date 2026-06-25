package loader

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func patchSchema(schema map[string]interface{}) {
	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		return
	}
	nullables := map[string]bool{}
	for key, val := range properties {
		obj, ok := val.(map[string]interface{})
		if !ok {
			panic(fmt.Errorf("Invalid `properties` object in schema: %v", val))
		}
		nodeType, nullable, typeErr := MainNodeType(obj)
		if typeErr != nil {
			panic(typeErr)
		}
		if nullable {
			nullables[key] = true
		}
		if nodeType == "object" {
			obj["goJSONSchema"] = map[string]interface{}{"pointer": true}
			patchSchema(obj)
		} else if nodeType == "array" {
			items, ok := obj["items"].(map[string]interface{})
			if !ok {
				continue
			}
			patchSchema(items)
		}
	}
	required, requiredOk := schema["required"].([]interface{})
	if requiredOk {
		newRequired := []interface{}{}
		for _, key := range required {
			keyStr, _ := key.(string)
			if _, nullable := nullables[keyStr]; !nullable {
				newRequired = append(newRequired, key)
			}
		}
		schema["required"] = newRequired
	}
}

func ExportSchemas(outputDir string) {
	InitSchemas()

	schemas := ListSchemas()

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	for _, id := range schemas {
		schema, err := GetSchema(id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting schema %s: %v\n", id, err)
			os.Exit(1)
		}

		patchSchema(schema)
		outData, err := json.MarshalIndent(schema, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling schema %s: %v\n", id, err)
			os.Exit(1)
		}

		outPath := filepath.Join(outputDir, string(id)+".json")
		if err := os.WriteFile(outPath, outData, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", outPath, err)
			os.Exit(1)
		}

		fmt.Printf("  %s -> %s\n", id, outPath)
	}

	fmt.Printf("\nExported %d schema(s) to %s\n", len(schemas), outputDir)
}
