package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/happycollision/ribbin/internal/testsafety"
)

func TestInitCreatesConfigWithSchema(t *testing.T) {
	_, tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Run init command
	err := runInit(initCmd, []string{})
	if err != nil {
		t.Fatalf("runInit failed: %v", err)
	}

	// Read the created config
	configPath := filepath.Join(tempDir, "ribbin.jsonc")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read created config: %v", err)
	}

	// Verify $schema is present
	if !strings.Contains(string(content), `"$schema"`) {
		t.Error("created config should contain $schema field")
	}

	// Verify it points to the latest schema URL
	if !strings.Contains(string(content), LatestSchemaURL) {
		t.Errorf("created config should contain latest schema URL %q", LatestSchemaURL)
	}
}

func TestLatestSchemaURLMatchesVersion(t *testing.T) {
	// This test ensures LatestSchemaURL is consistent with LatestSchemaVersion.
	// If someone updates the version but forgets the URL (or vice versa), this will catch it.
	expectedURL := "https://github.com/happycollision/ribbin/schemas/" + LatestSchemaVersion + "/ribbin.schema.json"
	if LatestSchemaURL != expectedURL {
		t.Errorf("LatestSchemaURL = %q, want %q (based on LatestSchemaVersion = %q)",
			LatestSchemaURL, expectedURL, LatestSchemaVersion)
	}
}

func TestDefaultConfigContainsSchemaAtStart(t *testing.T) {
	// The $schema should be the first property in the JSON for proper editor support
	lines := strings.Split(defaultConfig, "\n")

	foundSchema := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip empty lines and comments
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || trimmed == "{" {
			continue
		}
		// First non-comment, non-empty line should be $schema
		if strings.Contains(trimmed, `"$schema"`) {
			foundSchema = true
			break
		}
		// If we hit any other property first, fail
		if strings.HasPrefix(trimmed, `"`) {
			t.Errorf("expected $schema to be the first property, but found: %s", trimmed)
			break
		}
	}

	if !foundSchema {
		t.Error("$schema should be present near the start of defaultConfig")
	}
}
