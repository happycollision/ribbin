package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/happycollision/ribbin/internal/config"
	_ "github.com/happycollision/ribbin/internal/testsafety"
)

func TestDefaultConfigValidatesAgainstSchema(t *testing.T) {
	// Use ValidationStrict to catch any typos or invalid properties
	err := config.ValidateAgainstSchema([]byte(defaultConfig), config.ValidationStrict)
	if err != nil {
		t.Errorf("defaultConfig failed schema validation: %v", err)
	}
}

func TestExampleConfigValidatesAgainstSchema(t *testing.T) {
	// Use ValidationStrict to catch any typos or invalid properties
	err := config.ValidateAgainstSchema([]byte(ExampleConfig), config.ValidationStrict)
	if err != nil {
		t.Errorf("ExampleConfig failed schema validation: %v", err)
	}
}

func TestDefaultConfigPassesLooseValidation(t *testing.T) {
	// Loose validation should definitely pass
	err := config.ValidateAgainstSchema([]byte(defaultConfig), config.ValidationLoose)
	if err != nil {
		t.Errorf("defaultConfig failed loose schema validation: %v", err)
	}
}

func TestExampleConfigPassesLooseValidation(t *testing.T) {
	// Loose validation should definitely pass
	err := config.ValidateAgainstSchema([]byte(ExampleConfig), config.ValidationLoose)
	if err != nil {
		t.Errorf("ExampleConfig failed loose schema validation: %v", err)
	}
}

func TestConfigValidateCommand(t *testing.T) {
	_, tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create a valid config file
	configPath := filepath.Join(tempDir, "ribbin.jsonc")
	validConfig := `{
		"$schema": "https://github.com/happycollision/ribbin/schemas/v1/ribbin.schema.json",
		"wrappers": {
			"npm": {
				"action": "block",
				"message": "Use pnpm instead"
			}
		}
	}`
	if err := os.WriteFile(configPath, []byte(validConfig), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Run validate command - should succeed
	err := runConfigValidate(configValidateCmd, []string{configPath})
	if err != nil {
		t.Errorf("runConfigValidate failed on valid config: %v", err)
	}
}

func TestConfigValidateCommandInvalidConfig(t *testing.T) {
	_, tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create an invalid config file (missing required "action" field)
	configPath := filepath.Join(tempDir, "ribbin.jsonc")
	invalidConfig := `{
		"wrappers": {
			"npm": {
				"message": "Use pnpm instead"
			}
		}
	}`
	if err := os.WriteFile(configPath, []byte(invalidConfig), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Run validate command - the function itself doesn't return error for invalid schema,
	// it prints and calls os.Exit(1). We test the underlying validation instead.
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read test config: %v", err)
	}

	errors, _ := config.ValidateAgainstSchemaWithDetails(content)
	if len(errors) == 0 {
		t.Error("expected validation errors for config missing required 'action' field")
	}
}
