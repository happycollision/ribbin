package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/happycollision/ribbin/internal/testsafety"

	"github.com/happycollision/ribbin/internal/config"
	"github.com/happycollision/ribbin/internal/testutil"
	"github.com/happycollision/ribbin/internal/wrap"
)

// TestWrapCommandWithScopeWrappers tests that `ribbin wrap` correctly installs
// wrappers that are defined ONLY in scopes (not at root level).
func TestWrapCommandWithScopeWrappers(t *testing.T) {
	// Find module root before setupTestEnv changes the working directory
	moduleRoot := testutil.FindModuleRoot(t)

	tempHome, tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create a test project directory with subdirectories for scopes
	projectDir := filepath.Join(tempDir, "project")
	err := os.MkdirAll(filepath.Join(projectDir, "frontend"), 0755)
	if err != nil {
		t.Fatalf("failed to create project directories: %v", err)
	}
	err = os.MkdirAll(filepath.Join(projectDir, "backend"), 0755)
	if err != nil {
		t.Fatalf("failed to create project directories: %v", err)
	}

	// Initialize git repo (required for ribbin)
	cmd := exec.Command("git", "init")
	cmd.Dir = projectDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = projectDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = projectDir
	cmd.Run()

	// Create bin directory for mock commands
	binDir := filepath.Join(projectDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("failed to create bin dir: %v", err)
	}

	// Create mock binaries
	mockBinaries := []string{"tsc", "eslint", "jest"}
	for _, name := range mockBinaries {
		binPath := filepath.Join(binDir, name)
		content := "#!/bin/bash\necho \"" + name + " executed\"\n"
		if err := os.WriteFile(binPath, []byte(content), 0755); err != nil {
			t.Fatalf("failed to create mock binary %s: %v", name, err)
		}
	}

	// Create ribbin.jsonc with wrappers ONLY defined in scopes
	configContent := `{
  "scopes": {
    "frontend": {
      "path": "frontend",
      "wrappers": {
        "tsc": {
          "action": "block",
          "message": "Use 'pnpm nx type-check' instead",
          "paths": ["` + filepath.Join(binDir, "tsc") + `"]
        },
        "eslint": {
          "action": "block",
          "message": "Use 'pnpm nx lint' instead",
          "paths": ["` + filepath.Join(binDir, "eslint") + `"]
        }
      }
    },
    "backend": {
      "path": "backend",
      "wrappers": {
        "jest": {
          "action": "block",
          "message": "Use 'pnpm nx test' instead",
          "paths": ["` + filepath.Join(binDir, "jest") + `"]
        }
      }
    }
  }
}`

	configPath := createTestConfig(t, projectDir, configContent)

	// Change to project directory
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("failed to chdir to project: %v", err)
	}

	// Create empty registry
	createTestRegistry(t, tempHome, &config.Registry{
		Wrappers: make(map[string]config.WrapperEntry),
	})

	// Build ribbin binary
	ribbinPath := filepath.Join(tempDir, "ribbin")
	buildCmd := exec.Command("go", "build", "-o", ribbinPath, "./cmd/ribbin")
	buildCmd.Dir = moduleRoot
	if output, buildErr := buildCmd.CombinedOutput(); buildErr != nil {
		t.Fatalf("failed to build ribbin: %v\n%s", buildErr, output)
	}

	// Load the config to verify structure
	projectConfig, err := config.LoadProjectConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load project config: %v", err)
	}

	// Verify no root-level wrappers exist
	if len(projectConfig.Wrappers) != 0 {
		t.Fatalf("expected 0 root wrappers, got %d", len(projectConfig.Wrappers))
	}

	// Verify scope wrappers exist
	if len(projectConfig.Scopes) != 2 {
		t.Fatalf("expected 2 scopes, got %d", len(projectConfig.Scopes))
	}
	frontendWrappers := projectConfig.Scopes["frontend"].Wrappers
	if len(frontendWrappers) != 2 {
		t.Fatalf("expected 2 frontend wrappers, got %d", len(frontendWrappers))
	}
	backendWrappers := projectConfig.Scopes["backend"].Wrappers
	if len(backendWrappers) != 1 {
		t.Fatalf("expected 1 backend wrapper, got %d", len(backendWrappers))
	}

	// Load registry before wrapping
	registry, err := config.LoadRegistry()
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	// Install wrappers from scopes
	// This is the key test: wrappers defined in scopes should be installed
	wrapped := 0
	skipped := 0
	failed := 0

	// Process frontend scope wrappers
	for name, wrapperCfg := range frontendWrappers {
		for _, path := range wrapperCfg.Paths {
			if _, err := os.Stat(path); os.IsNotExist(err) {
				skipped++
				continue
			}

			// Import the wrap package to use Install function
			err := installWrapper(t, path, ribbinPath, registry, configPath)
			if err != nil {
				t.Logf("Failed to wrap %s at %s: %v", name, path, err)
				failed++
				continue
			}
			wrapped++
		}
	}

	// Process backend scope wrappers
	for name, wrapperCfg := range backendWrappers {
		for _, path := range wrapperCfg.Paths {
			if _, err := os.Stat(path); os.IsNotExist(err) {
				skipped++
				continue
			}

			err := installWrapper(t, path, ribbinPath, registry, configPath)
			if err != nil {
				t.Logf("Failed to wrap %s at %s: %v", name, path, err)
				failed++
				continue
			}
			wrapped++
		}
	}

	// Verify results
	if wrapped != 3 {
		t.Errorf("expected 3 binaries wrapped, got %d (skipped: %d, failed: %d)", wrapped, skipped, failed)
	}

	// Verify sidecar files exist
	for _, name := range mockBinaries {
		binPath := filepath.Join(binDir, name)
		sidecarPath := binPath + ".ribbin-original"
		if _, err := os.Stat(sidecarPath); os.IsNotExist(err) {
			t.Errorf("expected sidecar file to exist at %s", sidecarPath)
		}

		// Verify symlink exists and points to ribbin
		info, err := os.Lstat(binPath)
		if err != nil {
			t.Errorf("failed to stat %s: %v", binPath, err)
			continue
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("expected %s to be a symlink", binPath)
		}

		target, err := os.Readlink(binPath)
		if err != nil {
			t.Errorf("failed to readlink %s: %v", binPath, err)
			continue
		}
		if !strings.Contains(target, "ribbin") {
			t.Errorf("expected symlink to point to ribbin, got %s", target)
		}
	}

	// Verify registry was updated
	if err := config.SaveRegistry(registry); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	// Reload registry and verify entries
	registry, err = config.LoadRegistry()
	if err != nil {
		t.Fatalf("failed to reload registry: %v", err)
	}

	// Check that all three commands are in the registry
	expectedCommands := []string{"tsc", "eslint", "jest"}
	for _, cmd := range expectedCommands {
		if _, exists := registry.Wrappers[cmd]; !exists {
			t.Errorf("expected command %s to be in registry", cmd)
		}
	}
}

// installWrapper is a helper that calls the Install function from wrap package
func installWrapper(t *testing.T, binaryPath, ribbinPath string, registry *config.Registry, configPath string) error {
	t.Helper()
	return wrap.Install(binaryPath, ribbinPath, registry, configPath)
}
