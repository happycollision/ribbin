package internal

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/happycollision/ribbin/internal/testsafety"

	"github.com/happycollision/ribbin/internal/testutil"
)

// TestUnwrapWithRelativePathsFromDifferentDir tests that unwrap works correctly
// when the config file contains relative paths and unwrap is run from a different
// directory than where the config file is located.
//
// This is a regression test for: https://github.com/happycollision/ribbin/issues/XXX
//
// Scenario:
//   - Config file at /projects/ribbin.jsonc contains paths like "./subproject/node_modules/.bin/tsc"
//   - User wraps the tool from /projects directory (works fine)
//   - User later runs unwrap from /projects/subproject directory (fails because
//     relative path "./subproject/..." doesn't exist from that directory)
func TestUnwrapWithRelativePathsFromDifferentDir(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	env.SetPathWithBinDir()

	// Create directory structure:
	//   /tmp/project/           <- config will be here
	//   /tmp/project/subproject/
	//   /tmp/project/subproject/node_modules/.bin/  <- binary will be here
	subprojectDir := filepath.Join(env.ProjectDir, "subproject")
	nodeModulesBin := filepath.Join(subprojectDir, "node_modules", ".bin")
	if err := os.MkdirAll(nodeModulesBin, 0755); err != nil {
		t.Fatalf("failed to create node_modules/.bin: %v", err)
	}

	// Initialize git repo at project level (required for ribbin)
	env.InitGitRepo(env.ProjectDir)

	// Create mock tsc binary in subproject/node_modules/.bin
	tscPath := filepath.Join(nodeModulesBin, "tsc")
	originalContent := "#!/bin/sh\necho 'tsc: original version'\nexit 0\n"
	if err := os.WriteFile(tscPath, []byte(originalContent), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}
	t.Logf("Created tsc at: %s", tscPath)

	// Build ribbin binary
	env.BuildRibbin("")

	// Create config with RELATIVE path at project root
	// The path is relative to where the config file is located
	relativePath := "./subproject/node_modules/.bin/tsc"
	configContent := `{
  "wrappers": {
    "tsc": {
      "action": "block",
      "message": "Use project script instead",
      "paths": ["` + relativePath + `"]
    }
  }
}`
	configPath := filepath.Join(env.ProjectDir, "ribbin.jsonc")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to create config: %v", err)
	}
	t.Logf("Created config at: %s with relative path: %s", configPath, relativePath)

	env.GitAdd(env.ProjectDir, "ribbin.jsonc")
	env.GitCommit(env.ProjectDir, "Add config")

	// Step 1: Wrap the binary from the project directory (where the config is)
	t.Log("Step 1: Running 'ribbin wrap' from project directory...")
	output := env.MustRunRibbin(env.ProjectDir, "wrap")
	t.Logf("Wrap output: %s", output)

	// Verify wrapping succeeded
	sidecarPath := tscPath + ".ribbin-original"
	env.AssertFileExists(sidecarPath)

	info, err := os.Lstat(tscPath)
	if err != nil {
		t.Fatalf("failed to stat tsc: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("expected tsc to be a symlink after wrapping")
	}
	t.Log("Wrapping succeeded - tsc is now a symlink with sidecar")

	// Step 2: Run unwrap from a DIFFERENT directory (the subproject)
	// This is where the bug manifests - the relative path "./subproject/..."
	// will be interpreted relative to the current working directory, not
	// relative to the config file.
	t.Log("Step 2: Running 'ribbin unwrap' from SUBPROJECT directory...")
	output, err = env.RunRibbin(subprojectDir, "unwrap")
	t.Logf("Unwrap output: %s", output)

	// The unwrap should succeed regardless of which directory we run it from
	if err != nil {
		t.Errorf("unwrap failed when run from different directory: %v", err)
	}

	// Verify the binary was properly restored
	env.AssertFileNotExists(sidecarPath)
	env.AssertNotSymlink(tscPath)

	// Verify the binary still works
	content, err := os.ReadFile(tscPath)
	if err != nil {
		t.Fatalf("failed to read restored tsc: %v", err)
	}
	if !testutil.Contains(string(content), "original version") {
		t.Error("expected restored binary to be the original version")
	}

	// Verify registry is clean
	registry := env.LoadRegistry()
	if _, exists := registry.Wrappers["tsc"]; exists {
		t.Error("expected tsc to be removed from registry after unwrap")
	}

	t.Log("Test completed successfully!")
}

// TestWrapStoresAbsolutePaths verifies that when wrapping with relative paths
// in the config, the registry stores absolute paths so unwrap works from any directory.
func TestWrapStoresAbsolutePaths(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	env.SetPathWithBinDir()

	// Create directory structure
	subprojectDir := filepath.Join(env.ProjectDir, "subproject")
	nodeModulesBin := filepath.Join(subprojectDir, "node_modules", ".bin")
	if err := os.MkdirAll(nodeModulesBin, 0755); err != nil {
		t.Fatalf("failed to create node_modules/.bin: %v", err)
	}

	env.InitGitRepo(env.ProjectDir)

	// Create mock binary
	tscPath := filepath.Join(nodeModulesBin, "tsc")
	if err := os.WriteFile(tscPath, []byte("#!/bin/sh\necho tsc\n"), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	env.BuildRibbin("")

	// Create config with relative path
	relativePath := "./subproject/node_modules/.bin/tsc"
	configContent := `{
  "wrappers": {
    "tsc": {
      "action": "block",
      "message": "Use project script instead",
      "paths": ["` + relativePath + `"]
    }
  }
}`
	configPath := filepath.Join(env.ProjectDir, "ribbin.jsonc")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to create config: %v", err)
	}

	env.GitAdd(env.ProjectDir, "ribbin.jsonc")
	env.GitCommit(env.ProjectDir, "Add config")

	// Wrap the binary
	env.MustRunRibbin(env.ProjectDir, "wrap")

	// Load the registry and verify the path is stored as absolute
	registry := env.LoadRegistry()

	entry, exists := registry.Wrappers["tsc"]
	if !exists {
		t.Fatal("expected tsc entry in registry")
	}

	// The stored path should be absolute, not relative
	if !filepath.IsAbs(entry.Original) {
		t.Errorf("expected registry to store absolute path, got relative: %s", entry.Original)
	}

	// The stored path should resolve to the actual binary location
	expectedAbsPath := tscPath
	if entry.Original != expectedAbsPath {
		t.Errorf("expected registry path to be %s, got %s", expectedAbsPath, entry.Original)
	}

	t.Logf("Registry stores path as: %s", entry.Original)
}
