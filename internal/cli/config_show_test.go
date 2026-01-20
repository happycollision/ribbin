package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigShowCommand_NoConfig(t *testing.T) {
	_, tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Don't create any config file
	_ = tempDir

	// Reset flags
	configShowJSON = false
	configShowCommand = ""

	err := runConfigShow(configShowCmd, []string{})
	if err == nil {
		t.Fatal("expected error when no config exists")
	}
	if !strings.Contains(err.Error(), "No ribbin.toml found") {
		t.Errorf("error = %q, want to contain 'No ribbin.toml found'", err.Error())
	}
}

func TestConfigShowCommand_RootShimsOnly(t *testing.T) {
	_, tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create a simple config with root shims only
	configContent := `
[shims.npm]
action = "block"
message = "Use pnpm instead"

[shims.cat]
action = "warn"
message = "Consider using bat"
`
	createTestConfig(t, tempDir, configContent)

	// Reset flags
	configShowJSON = false
	configShowCommand = ""

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runConfigShow(configShowCmd, []string{})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Fatalf("runConfigShow error = %v", err)
	}

	// Check output contains expected elements
	if !strings.Contains(output, "Config:") {
		t.Error("output should contain 'Config:'")
	}
	if !strings.Contains(output, "ribbin.toml") {
		t.Error("output should contain config path")
	}
	if !strings.Contains(output, "Scope:  (root)") {
		t.Error("output should show (root) when no scope matches")
	}
	if !strings.Contains(output, "npm") {
		t.Error("output should contain npm shim")
	}
	if !strings.Contains(output, "block") {
		t.Error("output should contain block action")
	}
	if !strings.Contains(output, "Use pnpm instead") {
		t.Error("output should contain npm message")
	}
}

func TestConfigShowCommand_ScopeMatching(t *testing.T) {
	_, tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create a config with scopes
	configContent := `
[shims.cat]
action = "warn"
message = "root cat"

[scopes.frontend]
path = "apps/frontend"
extends = ["root"]

[scopes.frontend.shims.npm]
action = "block"
message = "Use pnpm"

[scopes.frontend.shims.cat]
action = "block"
message = "frontend cat override"
`
	createTestConfig(t, tempDir, configContent)

	// Create the frontend directory and cd into it
	frontendDir := filepath.Join(tempDir, "apps", "frontend")
	if err := os.MkdirAll(frontendDir, 0755); err != nil {
		t.Fatalf("failed to create frontend dir: %v", err)
	}
	if err := os.Chdir(frontendDir); err != nil {
		t.Fatalf("failed to chdir to frontend: %v", err)
	}

	// Reset flags
	configShowJSON = false
	configShowCommand = ""

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runConfigShow(configShowCmd, []string{})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Fatalf("runConfigShow error = %v", err)
	}

	// Check that frontend scope is matched
	if !strings.Contains(output, "Scope:  frontend") {
		t.Errorf("output should show frontend scope, got: %s", output)
	}

	// Check that npm shim is shown (from frontend scope)
	if !strings.Contains(output, "npm") {
		t.Error("output should contain npm shim")
	}

	// Check that cat shows the overridden version
	if !strings.Contains(output, "frontend cat override") {
		t.Error("output should show overridden cat message")
	}

	// Check that override tracking is shown
	if !strings.Contains(output, "overrides") {
		t.Error("output should show override information")
	}
}

func TestConfigShowCommand_JSONOutput(t *testing.T) {
	_, tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	configContent := `
[shims.npm]
action = "block"
message = "Use pnpm"
`
	createTestConfig(t, tempDir, configContent)

	// Set JSON flag
	configShowJSON = true
	configShowCommand = ""
	defer func() { configShowJSON = false }()

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runConfigShow(configShowCmd, []string{})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Fatalf("runConfigShow error = %v", err)
	}

	// Parse as JSON
	var result configShowOutput
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nOutput: %s", err, output)
	}

	// Check structure
	if result.ConfigPath == "" {
		t.Error("config_path should not be empty")
	}
	if len(result.Shims) != 1 {
		t.Errorf("expected 1 shim, got %d", len(result.Shims))
	}

	npmShim, ok := result.Shims["npm"]
	if !ok {
		t.Fatal("expected npm shim in output")
	}
	if npmShim.Action != "block" {
		t.Errorf("npm action = %q, want %q", npmShim.Action, "block")
	}
	if npmShim.Message != "Use pnpm" {
		t.Errorf("npm message = %q, want %q", npmShim.Message, "Use pnpm")
	}
	if npmShim.Source.Fragment != "root" {
		t.Errorf("npm source fragment = %q, want %q", npmShim.Source.Fragment, "root")
	}
}

func TestConfigShowCommand_CommandFilter(t *testing.T) {
	_, tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	configContent := `
[shims.npm]
action = "block"
message = "Use pnpm"

[shims.cat]
action = "warn"
message = "Use bat"
`
	createTestConfig(t, tempDir, configContent)

	// Set command filter
	configShowJSON = false
	configShowCommand = "npm"
	defer func() { configShowCommand = "" }()

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runConfigShow(configShowCmd, []string{})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Fatalf("runConfigShow error = %v", err)
	}

	// Should contain npm
	if !strings.Contains(output, "npm") {
		t.Error("output should contain npm")
	}

	// Should NOT contain cat (filtered out)
	if strings.Contains(output, "cat") {
		t.Error("output should not contain cat when filtering by command")
	}
}

func TestConfigShowCommand_CommandNotFound(t *testing.T) {
	_, tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	configContent := `
[shims.npm]
action = "block"
message = "Use pnpm"
`
	createTestConfig(t, tempDir, configContent)

	// Set command filter to non-existent command
	configShowJSON = false
	configShowCommand = "nonexistent"
	defer func() { configShowCommand = "" }()

	err := runConfigShow(configShowCmd, []string{})
	if err == nil {
		t.Fatal("expected error when command not found")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want to contain 'not found'", err.Error())
	}
}

func TestConfigShowCommand_NoShims(t *testing.T) {
	_, tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create a config with no shims
	configContent := `# Empty config
`
	createTestConfig(t, tempDir, configContent)

	// Reset flags
	configShowJSON = false
	configShowCommand = ""

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runConfigShow(configShowCmd, []string{})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Fatalf("runConfigShow error = %v", err)
	}

	if !strings.Contains(output, "No effective shims configured") {
		t.Errorf("output should indicate no shims, got: %s", output)
	}
}

func TestConfigShowCommand_ProvenanceChain(t *testing.T) {
	_, tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create a config with multiple layers of extends
	configContent := `
[shims.cat]
action = "warn"
message = "root cat"

[scopes.hardened]
[scopes.hardened.shims.cat]
action = "block"
message = "hardened cat"

[scopes.frontend]
path = "apps/frontend"
extends = ["root", "root.hardened"]

[scopes.frontend.shims.cat]
action = "redirect"
message = "frontend cat"
redirect = "bat"
`
	createTestConfig(t, tempDir, configContent)

	// Create the frontend directory and cd into it
	frontendDir := filepath.Join(tempDir, "apps", "frontend")
	if err := os.MkdirAll(frontendDir, 0755); err != nil {
		t.Fatalf("failed to create frontend dir: %v", err)
	}
	if err := os.Chdir(frontendDir); err != nil {
		t.Fatalf("failed to chdir to frontend: %v", err)
	}

	// Test JSON output for detailed provenance
	configShowJSON = true
	configShowCommand = "cat"
	defer func() {
		configShowJSON = false
		configShowCommand = ""
	}()

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runConfigShow(configShowCmd, []string{})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Fatalf("runConfigShow error = %v", err)
	}

	// Parse as JSON
	var result configShowOutput
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nOutput: %s", err, output)
	}

	// Check cat shim
	catShim, ok := result.Shims["cat"]
	if !ok {
		t.Fatal("expected cat shim")
	}

	// Should be from frontend scope
	if catShim.Source.Fragment != "root.frontend" {
		t.Errorf("cat source fragment = %q, want %q", catShim.Source.Fragment, "root.frontend")
	}

	// Should have override chain
	if catShim.Source.Overrode == nil {
		t.Fatal("cat should have overrode set")
	}
	if catShim.Source.Overrode.Fragment != "root.hardened" {
		t.Errorf("cat first overrode = %q, want %q", catShim.Source.Overrode.Fragment, "root.hardened")
	}

	// Should have another level of override
	if catShim.Source.Overrode.Overrode == nil {
		t.Fatal("cat should have second level overrode set")
	}
	if catShim.Source.Overrode.Overrode.Fragment != "root" {
		t.Errorf("cat second overrode = %q, want %q", catShim.Source.Overrode.Overrode.Fragment, "root")
	}
}
