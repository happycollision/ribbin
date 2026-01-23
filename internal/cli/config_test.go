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

// Tests for explicit config path arguments

func TestConfigListWithExplicitPath(t *testing.T) {
	_, tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create a config file in a subdirectory (not auto-discoverable from tempDir)
	subDir := filepath.Join(tempDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	configPath := filepath.Join(subDir, "ribbin.jsonc")
	configContent := `{
		"wrappers": {
			"npm": {
				"action": "block",
				"message": "Use pnpm"
			}
		}
	}`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Reset flags
	configListJSON = false
	configListCommand = ""

	// Test with explicit path
	err := runConfigList(configListCmd, []string{configPath})
	if err != nil {
		t.Errorf("runConfigList with explicit path failed: %v", err)
	}
}

func TestConfigListWithNonexistentPath(t *testing.T) {
	_, _, cleanup := setupTestEnv(t)
	defer cleanup()

	// Reset flags
	configListJSON = false
	configListCommand = ""

	// Test with nonexistent path
	err := runConfigList(configListCmd, []string{"/nonexistent/ribbin.jsonc"})
	if err == nil {
		t.Error("expected error for nonexistent config path")
	}
	if err != nil && !stringContains(err.Error(), "config file not found") {
		t.Errorf("error = %q, want to contain 'config file not found'", err.Error())
	}
}

func TestConfigShowWithExplicitPath(t *testing.T) {
	_, tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create a config file in a subdirectory
	subDir := filepath.Join(tempDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	configPath := filepath.Join(subDir, "ribbin.jsonc")
	configContent := `{
		"wrappers": {
			"npm": {
				"action": "block",
				"message": "Use pnpm"
			}
		}
	}`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Reset flags
	configShowJSON = false
	configShowCommand = ""

	// Test with explicit path
	err := runConfigShow(configShowCmd, []string{configPath})
	if err != nil {
		t.Errorf("runConfigShow with explicit path failed: %v", err)
	}
}

func TestConfigShowWithNonexistentPath(t *testing.T) {
	_, _, cleanup := setupTestEnv(t)
	defer cleanup()

	// Reset flags
	configShowJSON = false
	configShowCommand = ""

	// Test with nonexistent path
	err := runConfigShow(configShowCmd, []string{"/nonexistent/ribbin.jsonc"})
	if err == nil {
		t.Error("expected error for nonexistent config path")
	}
	if err != nil && !stringContains(err.Error(), "config file not found") {
		t.Errorf("error = %q, want to contain 'config file not found'", err.Error())
	}
}

func TestConfigAddWithExplicitPath(t *testing.T) {
	_, tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create a config file in a subdirectory
	subDir := filepath.Join(tempDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	configPath := filepath.Join(subDir, "ribbin.jsonc")
	configContent := `{
		"wrappers": {}
	}`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Reset flags
	addAction = "block"
	addMessage = "Test message"
	addRedirect = ""
	addPaths = nil
	addCreateScript = false
	defer func() {
		addAction = ""
		addMessage = ""
	}()

	// Test with explicit path and command (2 args)
	err := runConfigAdd(configAddCmd, []string{configPath, "testcmd"})
	if err != nil {
		t.Errorf("runConfigAdd with explicit path failed: %v", err)
	}

	// Verify the command was added
	cfg, err := config.LoadProjectConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	if _, exists := cfg.Wrappers["testcmd"]; !exists {
		t.Error("expected testcmd to be added to config")
	}
}

func TestConfigAddWithNonexistentPath(t *testing.T) {
	_, _, cleanup := setupTestEnv(t)
	defer cleanup()

	// Reset flags
	addAction = "block"
	addMessage = "Test"
	defer func() {
		addAction = ""
		addMessage = ""
	}()

	// Test with nonexistent path
	err := runConfigAdd(configAddCmd, []string{"/nonexistent/ribbin.jsonc", "testcmd"})
	if err == nil {
		t.Error("expected error for nonexistent config path")
	}
	if err != nil && !stringContains(err.Error(), "config file not found") {
		t.Errorf("error = %q, want to contain 'config file not found'", err.Error())
	}
}

func TestConfigEditWithExplicitPath(t *testing.T) {
	_, tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create a config file with an existing command
	subDir := filepath.Join(tempDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	configPath := filepath.Join(subDir, "ribbin.jsonc")
	configContent := `{
		"wrappers": {
			"npm": {
				"action": "block",
				"message": "Original message"
			}
		}
	}`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Reset flags
	editAction = ""
	editMessage = "Updated message"
	editRedirect = ""
	editPaths = nil
	editClearMessage = false
	editClearPaths = false
	editClearRedirect = false
	defer func() {
		editMessage = ""
	}()

	// Mark the message flag as changed
	configEditCmd.Flags().Set("message", "Updated message")
	defer configEditCmd.Flags().Set("message", "")

	// Test with explicit path and command (2 args)
	err := runConfigEdit(configEditCmd, []string{configPath, "npm"})
	if err != nil {
		t.Errorf("runConfigEdit with explicit path failed: %v", err)
	}

	// Verify the command was updated
	cfg, err := config.LoadProjectConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	if cfg.Wrappers["npm"].Message != "Updated message" {
		t.Errorf("message = %q, want %q", cfg.Wrappers["npm"].Message, "Updated message")
	}
}

func TestConfigEditWithNonexistentPath(t *testing.T) {
	_, _, cleanup := setupTestEnv(t)
	defer cleanup()

	// Reset flags and mark message as changed
	editMessage = "Test"
	configEditCmd.Flags().Set("message", "Test")
	defer func() {
		editMessage = ""
		configEditCmd.Flags().Set("message", "")
	}()

	// Test with nonexistent path
	err := runConfigEdit(configEditCmd, []string{"/nonexistent/ribbin.jsonc", "npm"})
	if err == nil {
		t.Error("expected error for nonexistent config path")
	}
	if err != nil && !stringContains(err.Error(), "config file not found") {
		t.Errorf("error = %q, want to contain 'config file not found'", err.Error())
	}
}

func TestConfigRemoveWithExplicitPath(t *testing.T) {
	_, tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create a config file with a command to remove
	subDir := filepath.Join(tempDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	configPath := filepath.Join(subDir, "ribbin.jsonc")
	configContent := `{
		"wrappers": {
			"npm": {
				"action": "block",
				"message": "Use pnpm"
			}
		}
	}`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Reset flags
	configRemoveForce = true
	defer func() {
		configRemoveForce = false
	}()

	// Test with explicit path and command (2 args)
	err := runConfigRemove(configRemoveCmd, []string{configPath, "npm"})
	if err != nil {
		t.Errorf("runConfigRemove with explicit path failed: %v", err)
	}

	// Verify the command was removed
	cfg, err := config.LoadProjectConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	if _, exists := cfg.Wrappers["npm"]; exists {
		t.Error("expected npm to be removed from config")
	}
}

func TestConfigRemoveWithNonexistentPath(t *testing.T) {
	_, _, cleanup := setupTestEnv(t)
	defer cleanup()

	// Reset flags
	configRemoveForce = true
	defer func() {
		configRemoveForce = false
	}()

	// Test with nonexistent path
	err := runConfigRemove(configRemoveCmd, []string{"/nonexistent/ribbin.jsonc", "npm"})
	if err == nil {
		t.Error("expected error for nonexistent config path")
	}
	if err != nil && !stringContains(err.Error(), "config file not found") {
		t.Errorf("error = %q, want to contain 'config file not found'", err.Error())
	}
}

// Helper function for checking if string contains substring
func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
