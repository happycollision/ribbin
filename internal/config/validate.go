package config

import (
	"encoding/json"
	"fmt"
	"strings"

	_ "embed"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/tailscale/hujson"
)

// LooseSchemaBytes contains the loose schema that allows additional properties.
// This is the schema intended for downstream users.
//
//go:embed schemas/v1/ribbin.schema.json
var LooseSchemaBytes []byte

// StrictSchemaBytes contains the strict schema that disallows additional properties.
// This is used for internal validation to catch typos.
//
//go:embed schemas/v1/ribbin.schema.strict.json
var StrictSchemaBytes []byte

// ValidationMode determines how strictly the config is validated.
type ValidationMode int

const (
	// ValidationLoose allows additional properties not in the schema.
	ValidationLoose ValidationMode = iota
	// ValidationStrict fails on unknown properties.
	ValidationStrict
)

// ValidateAgainstSchema validates JSONC content against the ribbin schema.
// Returns nil if valid, or a detailed error with all validation failures.
func ValidateAgainstSchema(jsoncContent []byte, mode ValidationMode) error {
	errors, warnings := ValidateAgainstSchemaWithDetails(jsoncContent)

	if len(errors) > 0 {
		return fmt.Errorf("validation errors:\n  - %s", strings.Join(errors, "\n  - "))
	}

	if mode == ValidationStrict && len(warnings) > 0 {
		return fmt.Errorf("strict validation warnings (unknown properties):\n  - %s", strings.Join(warnings, "\n  - "))
	}

	return nil
}

// ValidateAgainstSchemaWithDetails returns both hard errors and warnings.
// Hard errors are loose validation failures (schema violations).
// Warnings are strict-only failures (unknown properties).
func ValidateAgainstSchemaWithDetails(jsoncContent []byte) (errors []string, warnings []string) {
	// Strip comments using hujson
	standardized, err := hujson.Standardize(jsoncContent)
	if err != nil {
		return []string{fmt.Sprintf("failed to parse JSONC: %v", err)}, nil
	}

	// Parse JSON to interface{} for validation
	var doc interface{}
	if err := json.Unmarshal(standardized, &doc); err != nil {
		return []string{fmt.Sprintf("failed to parse JSON: %v", err)}, nil
	}

	// First, validate with loose schema (original)
	looseSchema, err := jsonschema.UnmarshalJSON(strings.NewReader(string(LooseSchemaBytes)))
	if err != nil {
		return []string{fmt.Sprintf("failed to parse loose schema: %v", err)}, nil
	}

	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("ribbin.schema.json", looseSchema); err != nil {
		return []string{fmt.Sprintf("failed to add loose schema resource: %v", err)}, nil
	}

	schema, err := compiler.Compile("ribbin.schema.json")
	if err != nil {
		return []string{fmt.Sprintf("failed to compile loose schema: %v", err)}, nil
	}

	looseErr := schema.Validate(doc)
	if looseErr != nil {
		// Extract error details
		if validationErr, ok := looseErr.(*jsonschema.ValidationError); ok {
			errors = extractValidationErrors(validationErr)
		} else {
			errors = []string{looseErr.Error()}
		}
		return errors, nil
	}

	// Now validate with strict schema to find unknown properties
	strictSchema, err := jsonschema.UnmarshalJSON(strings.NewReader(string(StrictSchemaBytes)))
	if err != nil {
		// If strict schema fails to parse, just return no warnings
		return nil, nil
	}

	strictCompiler := jsonschema.NewCompiler()
	if err := strictCompiler.AddResource("ribbin-strict.schema.json", strictSchema); err != nil {
		return nil, nil
	}

	strict, err := strictCompiler.Compile("ribbin-strict.schema.json")
	if err != nil {
		return nil, nil
	}

	strictErr := strict.Validate(doc)
	if strictErr != nil {
		if validationErr, ok := strictErr.(*jsonschema.ValidationError); ok {
			warnings = extractValidationErrors(validationErr)
		} else {
			warnings = []string{strictErr.Error()}
		}
	}

	return nil, warnings
}

// extractValidationErrors recursively extracts error messages from a ValidationError.
func extractValidationErrors(err *jsonschema.ValidationError) []string {
	var messages []string

	// If this error has causes, recurse into them
	if len(err.Causes) > 0 {
		for _, cause := range err.Causes {
			messages = append(messages, extractValidationErrors(cause)...)
		}
	} else {
		// Leaf error - format it nicely
		path := formatJSONPointer(err.InstanceLocation)
		messages = append(messages, fmt.Sprintf("%s: %s", path, err.Error()))
	}

	return messages
}

// formatJSONPointer converts a JSON Pointer path ([]string) to a readable string.
func formatJSONPointer(segments []string) string {
	if len(segments) == 0 {
		return "(root)"
	}
	return "/" + strings.Join(segments, "/")
}
