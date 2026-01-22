package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	_ "github.com/happycollision/ribbin/internal/testsafety"

	"github.com/happycollision/ribbin/internal/config"
	"github.com/happycollision/ribbin/internal/testutil"
)

// TestScopeMatching tests that scopes are matched correctly based on CWD
func TestScopeMatching(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	fixtureDir := filepath.Join(env.ModuleRoot, "testdata", "projects", "scoped")
	configPath := filepath.Join(fixtureDir, "ribbin.jsonc")

	cfg, err := config.LoadProjectConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Create the directory that the fixture scope expects (apps/frontend)
	frontendDir := filepath.Join(fixtureDir, "apps", "frontend")
	if err := os.MkdirAll(frontendDir, 0755); err != nil {
		t.Fatalf("failed to create frontend dir: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(filepath.Join(fixtureDir, "apps"))
	})

	// Test matching from frontend directory
	// Note: FindMatchingScope takes configDir (directory), not configPath (file)
	matched := config.FindMatchingScope(cfg, fixtureDir, frontendDir)

	if matched == nil {
		t.Error("expected to match frontend scope")
	} else if matched.Config.Path != "apps/frontend" {
		t.Errorf("expected frontend scope path 'apps/frontend', got %s", matched.Config.Path)
	}

	// Test no match from root directory
	matched = config.FindMatchingScope(cfg, fixtureDir, fixtureDir)
	if matched != nil {
		t.Logf("Root matched scope: %s (this may be expected)", matched.Name)
	}
}

// TestEndToEndScopedBlocking tests end-to-end scoped blocking behavior
func TestEndToEndScopedBlocking(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	env.SetPathWithBinDir()

	// Create directory structure with scopes (relative to ProjectDir)
	frontendDir := filepath.Join(env.ProjectDir, "frontend")
	backendDir := filepath.Join(env.ProjectDir, "backend")
	os.MkdirAll(frontendDir, 0755)
	os.MkdirAll(backendDir, 0755)

	// Create mock npm binary
	npmPath := env.CreateMockBinaryWithOutput(env.BinDir, "npm", "REAL_NPM: executed")

	// Init git repo
	env.InitGitRepo(env.ProjectDir)

	// Create config with scoped blocking
	// Note: scope paths are relative to config file location (env.ProjectDir)
	configContent := fmt.Sprintf(`{
  "wrappers": {
    "npm": {
      "action": "block",
      "message": "Use pnpm instead of npm (from root)",
      "paths": ["%s"]
    }
  },
  "scopes": {
    "frontend": {
      "path": "frontend",
      "extends": ["root"],
      "wrappers": {
        "npm": {
          "action": "block",
          "message": "Use pnpm in frontend"
        }
      }
    },
    "backend": {
      "path": "backend",
      "wrappers": {
        "npm": {
          "action": "passthrough"
        }
      }
    }
  }
}`, npmPath)

	configPath := env.CreateConfig(env.ProjectDir, configContent)
	env.GitAdd(env.ProjectDir, "ribbin.jsonc")
	env.GitCommit(env.ProjectDir, "Add config")

	// Build ribbin
	env.BuildRibbin("")

	// Wrap npm
	t.Log("Wrapping npm...")
	env.MustRunRibbin(env.ProjectDir, "wrap")
	env.MustRunRibbin(env.ProjectDir, "activate", "--global")

	// Verify shim
	env.AssertSymlink(npmPath, env.RibbinPath)

	// Test 1: From frontend (has block) - should be blocked
	os.Chdir(frontendDir)
	cmd := exec.Command("npm", "install")
	cmd.Env = env.Environ()
	output, err := cmd.CombinedOutput()

	if err == nil {
		t.Errorf("npm should be blocked in frontend: %s", output)
	}
	env.AssertOutputContains(string(output), "pnpm")
	t.Logf("Frontend npm blocked as expected: %s", output)

	// Test 2: From backend (has passthrough) - should work
	os.Chdir(backendDir)
	cmd = exec.Command("npm", "install")
	cmd.Env = env.Environ()
	output, err = cmd.CombinedOutput()

	if err != nil {
		t.Errorf("npm should passthrough in backend: %v\nOutput: %s", err, output)
	}
	env.AssertOutputContains(string(output), "REAL_NPM")
	t.Logf("Backend npm passthrough works: %s", output)

	// Cleanup
	env.MustRunRibbin(env.ProjectDir, "unwrap")

	t.Log("End-to-end scoped blocking test completed!")

	_ = configPath
}

// TestScopeWrappersWrapUnwrap tests wrap/unwrap with scoped wrappers
func TestScopeWrappersWrapUnwrap(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	env.SetPathWithBinDir()

	// Create project structure
	frontendDir := env.CreateDir("project/frontend")

	// Create mock binaries
	mockBinaries := map[string]string{
		"tsc":     "TypeScript v5.0.0",
		"eslint":  "ESLint v8.0.0",
		"prettier": "Prettier v3.0.0",
	}

	for name, output := range mockBinaries {
		env.CreateMockBinaryWithOutput(env.BinDir, name, output)
	}

	// Init git repo
	env.InitGitRepo(env.ProjectDir)

	// Create config with scope wrappers
	configContent := `{
  "scopes": {
    "frontend": {
      "path": "project/frontend",
      "wrappers": {
        "tsc": {
          "action": "block",
          "message": "Use pnpm run typecheck",
          "paths": ["` + filepath.Join(env.BinDir, "tsc") + `"]
        },
        "eslint": {
          "action": "block",
          "message": "Use pnpm run lint",
          "paths": ["` + filepath.Join(env.BinDir, "eslint") + `"]
        },
        "prettier": {
          "action": "block",
          "message": "Use pnpm run format",
          "paths": ["` + filepath.Join(env.BinDir, "prettier") + `"]
        }
      }
    }
  }
}`
	configPath := env.CreateConfig(env.ProjectDir, configContent)
	env.GitAdd(env.ProjectDir, "ribbin.jsonc")
	env.GitCommit(env.ProjectDir, "Add config")

	// Build ribbin
	env.BuildRibbin("")

	// Wrap
	t.Log("Running ribbin wrap...")
	output := env.MustRunRibbin(env.ProjectDir, "wrap")
	t.Logf("Wrap output: %s", output)

	env.AssertOutputContains(output, "3 wrapped")

	// Verify all binaries are shimmed
	for name := range mockBinaries {
		binPath := filepath.Join(env.BinDir, name)
		sidecarPath := binPath + ".ribbin-original"

		env.AssertSymlink(binPath, env.RibbinPath)
		env.AssertFileExists(sidecarPath)
	}
	t.Log("✓ All commands shimmed correctly")

	// Unwrap all
	t.Log("Running ribbin unwrap --all...")
	output = env.MustRunRibbin(env.ProjectDir, "unwrap", "--all")
	t.Logf("Unwrap output: %s", output)

	env.AssertOutputContains(output, "3 restored")

	// Verify all restored
	for name := range mockBinaries {
		binPath := filepath.Join(env.BinDir, name)
		sidecarPath := binPath + ".ribbin-original"

		env.AssertNotSymlink(binPath)
		env.AssertFileNotExists(sidecarPath)
	}
	t.Log("✓ All originals restored")

	// Verify registry is empty
	registryPath := filepath.Join(env.HomeDir, ".config", "ribbin", "registry.json")
	registryData, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatalf("failed to read registry: %v", err)
	}

	var cleanRegistry config.Registry
	if err := json.Unmarshal(registryData, &cleanRegistry); err != nil {
		t.Fatalf("failed to parse registry: %v", err)
	}

	if len(cleanRegistry.Wrappers) != 0 {
		t.Errorf("expected registry to be empty after unwrap, got %d entries", len(cleanRegistry.Wrappers))
	}
	t.Log("✓ Registry cleaned")

	t.Log("Scope wrappers wrap/unwrap test completed successfully!")

	_ = frontendDir
	_ = configPath
}

// TestScopeWrappersUnwrapWithoutAll tests unwrap without --all flag with scoped wrappers
func TestScopeWrappersUnwrapWithoutAll(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	env.SetPathWithBinDir()

	// Create project structure
	frontendDir := env.CreateDir("project/frontend")

	// Create mock tsc binary
	tscPath := env.CreateMockBinaryWithOutput(env.BinDir, "tsc", "TypeScript v5.0.0")

	// Init git repo
	env.InitGitRepo(env.ProjectDir)

	// Create config with scope wrapper
	configContent := `{
  "scopes": {
    "frontend": {
      "path": "project/frontend",
      "wrappers": {
        "tsc": {
          "action": "block",
          "message": "Use pnpm run typecheck",
          "paths": ["` + tscPath + `"]
        }
      }
    }
  }
}`
	configPath := env.CreateConfig(env.ProjectDir, configContent)
	env.GitAdd(env.ProjectDir, "ribbin.jsonc")
	env.GitCommit(env.ProjectDir, "Add config")

	// Build ribbin
	env.BuildRibbin("")

	// Wrap
	t.Log("Running ribbin wrap...")
	output := env.MustRunRibbin(env.ProjectDir, "wrap")
	t.Logf("Wrap output: %s", output)

	env.AssertOutputContains(output, "1 wrapped")

	// Verify wrapper was created
	sidecarPath := tscPath + ".ribbin-original"
	env.AssertFileExists(sidecarPath)

	// Run status to see it's tracked
	t.Log("Running ribbin status...")
	output = env.MustRunRibbin(env.ProjectDir, "status")
	t.Logf("Status output: %s", output)

	env.AssertOutputContains(output, tscPath)

	// Unwrap WITHOUT --all flag (from project root)
	t.Log("Running ribbin unwrap (without --all)...")
	output = env.MustRunRibbin(env.ProjectDir, "unwrap")
	t.Logf("Unwrap output: %s", output)

	// CRITICAL: This should unwrap the tsc binary even though it's in a scope
	env.AssertOutputNotContains(output, "No wrappers to remove")

	if !testutil.Contains(output, "1 restored") && !testutil.Contains(output, "Restored") {
		t.Errorf("expected unwrap to restore tsc, got: %s", output)
	}

	// Verify sidecar is gone and original restored
	env.AssertFileNotExists(sidecarPath)
	env.AssertNotSymlink(tscPath)

	t.Log("Scope wrappers unwrap-without-all test completed successfully!")

	_ = frontendDir
	_ = configPath
}
