package config

import (
	"encoding/json"
	"reflect"
	"testing"

	_ "github.com/happycollision/ribbin/internal/testsafety"
)

func TestStrictSchemaOnlyAddsAdditionalPropertiesFalse(t *testing.T) {
	var loose map[string]interface{}
	var strict map[string]interface{}

	if err := json.Unmarshal(LooseSchemaBytes, &loose); err != nil {
		t.Fatalf("failed to parse loose schema: %v", err)
	}

	if err := json.Unmarshal(StrictSchemaBytes, &strict); err != nil {
		t.Fatalf("failed to parse strict schema: %v", err)
	}

	// Remove additionalProperties from strict schema recursively,
	// then compare. They should be identical except for metadata fields.
	normalizedStrict := removeAdditionalPropertiesFalse(strict)

	// Also need to update $id and title/description which are expected to differ
	normalizedStrict["$id"] = loose["$id"]
	normalizedStrict["title"] = loose["title"]
	normalizedStrict["description"] = loose["description"]

	if !reflect.DeepEqual(loose, normalizedStrict) {
		t.Error("strict schema differs from loose schema in ways other than additionalProperties, $id, title, and description")

		// Pretty print the differences for debugging
		looseJSON, _ := json.MarshalIndent(loose, "", "  ")
		strictJSON, _ := json.MarshalIndent(normalizedStrict, "", "  ")
		t.Logf("Loose schema:\n%s", looseJSON)
		t.Logf("Normalized strict schema:\n%s", strictJSON)
	}
}

// removeAdditionalPropertiesFalse recursively removes "additionalProperties": false
// from a schema object, returning a deep copy.
func removeAdditionalPropertiesFalse(obj map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for key, val := range obj {
		// Skip additionalProperties: false
		if key == "additionalProperties" {
			if boolVal, ok := val.(bool); ok && !boolVal {
				continue // Skip this key
			}
		}

		switch v := val.(type) {
		case map[string]interface{}:
			result[key] = removeAdditionalPropertiesFalse(v)
		case []interface{}:
			newArr := make([]interface{}, len(v))
			for i, item := range v {
				if itemMap, ok := item.(map[string]interface{}); ok {
					newArr[i] = removeAdditionalPropertiesFalse(itemMap)
				} else {
					newArr[i] = item
				}
			}
			result[key] = newArr
		default:
			result[key] = val
		}
	}

	return result
}

func TestValidationLooseAllowsExtraProperties(t *testing.T) {
	configWithExtra := []byte(`{
		"wrappers": {
			"npm": {
				"action": "block",
				"message": "Use pnpm",
				"customField": "should be allowed in loose mode"
			}
		},
		"customTopLevel": "also allowed"
	}`)

	err := ValidateAgainstSchema(configWithExtra, ValidationLoose)
	if err != nil {
		t.Errorf("loose validation should allow extra properties, got: %v", err)
	}
}

func TestValidationStrictRejectsExtraProperties(t *testing.T) {
	configWithExtra := []byte(`{
		"wrappers": {
			"npm": {
				"action": "block",
				"message": "Use pnpm",
				"customField": "should be rejected in strict mode"
			}
		}
	}`)

	err := ValidateAgainstSchema(configWithExtra, ValidationStrict)
	if err == nil {
		t.Error("strict validation should reject extra properties")
	}
}
