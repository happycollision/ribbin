package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/happycollision/ribbin/internal/testsafety"

	"github.com/happycollision/ribbin/internal/config"
	"github.com/happycollision/ribbin/internal/testutil"
)

// TestFindOrphanedSidecars tests finding orphaned sidecar files
// Uses the real ribbin workflow to create properly wrapped binaries,
// then simulates orphaned state by clearing the registry.
func TestFindOrphanedSidecars(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	env.SetPathWithBinDir()
	env.InitGitRepo(env.ProjectDir)

	// Create test binary
	testBinaryPath := env.CreateMockBinaryWithOutput(env.BinDir, "test-cmd", "original test-cmd")

	// Build ribbin
	env.BuildRibbin("")

	// Create config and wrap properly
	env.CreateBlockConfig(env.ProjectDir, "test-cmd", "Use 'proper-cmd' instead", []string{testBinaryPath})
	env.GitAdd(env.ProjectDir, "ribbin.jsonc")
	env.GitCommit(env.ProjectDir, "Add config")

	// Wrap the binary properly
	env.MustRunRibbin(env.ProjectDir, "wrap")

	// Verify wrapped state
	sidecarPath := testBinaryPath + ".ribbin-original"
	metaPath := testBinaryPath + ".ribbin-meta"
	env.AssertFileExists(sidecarPath)
	env.AssertFileExists(metaPath)

	// Now simulate orphaned state by clearing the registry
	registryPath := filepath.Join(env.HomeDir, ".config", "ribbin", "registry.json")
	os.WriteFile(registryPath, []byte(`{"wrappers":{},"shell_activations":{},"config_activations":{},"global_active":false}`), 0644)

	// Run 'ribbin find <dir>' to find orphaned sidecars
	output, _ := env.RunRibbin(env.ProjectDir, "find", env.BinDir)
	t.Logf("Find output: %s", output)

	// Should find the orphaned sidecar
	env.AssertOutputContains(output, "test-cmd")

	// Now test cleanup with 'ribbin unwrap --all --find'
	output = env.MustRunRibbin(env.ProjectDir, "unwrap", "--all", "--find")
	t.Logf("Unwrap --all --find output: %s", output)

	// Verify sidecar is removed
	env.AssertFileNotExists(sidecarPath)

	// Verify metadata is removed
	env.AssertFileNotExists(metaPath)

	// Verify original is restored
	env.AssertNotSymlink(testBinaryPath)
	env.AssertFileExists(testBinaryPath)

	t.Log("Find orphaned sidecars test completed!")
}

// TestStatusFindStatusFlow tests the status -> find -> fix flow
func TestStatusFindStatusFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	env.SetPathWithBinDir()

	// Create test binaries
	bins := []string{"cmd1", "cmd2", "cmd3"}
	paths := make(map[string]string)
	for _, name := range bins {
		paths[name] = env.CreateMockBinaryWithOutput(env.BinDir, name, name+" output")
	}

	// Init git repo
	env.InitGitRepo(env.ProjectDir)

	// Create config for all binaries
	configContent := `{
  "wrappers": {
    "cmd1": {
      "action": "block",
      "message": "blocked",
      "paths": ["` + paths["cmd1"] + `"]
    },
    "cmd2": {
      "action": "block",
      "message": "blocked",
      "paths": ["` + paths["cmd2"] + `"]
    },
    "cmd3": {
      "action": "block",
      "message": "blocked",
      "paths": ["` + paths["cmd3"] + `"]
    }
  }
}`
	configPath := env.CreateConfig(env.ProjectDir, configContent)
	env.GitAdd(env.ProjectDir, "ribbin.jsonc")
	env.GitCommit(env.ProjectDir, "Add config")

	// Build ribbin
	env.BuildRibbin("")

	// Step 1: Check status (should show nothing wrapped)
	t.Log("Step 1: Initial status check...")
	output := env.MustRunRibbin(env.ProjectDir, "status")
	t.Logf("Initial status: %s", output)

	// Step 2: Wrap all
	t.Log("Step 2: Wrapping all...")
	output = env.MustRunRibbin(env.ProjectDir, "wrap")
	t.Logf("Wrap output: %s", output)

	// Step 3: Check status (should show all wrapped)
	t.Log("Step 3: Status after wrap...")
	output = env.MustRunRibbin(env.ProjectDir, "status")
	t.Logf("Status after wrap: %s", output)

	for _, name := range bins {
		env.AssertOutputContains(output, name)
	}

	// Step 4: Simulate orphaned state - remove cmd2 from registry but keep files
	t.Log("Step 4: Simulating orphaned state for cmd2...")
	registryPath := filepath.Join(env.HomeDir, ".config", "ribbin", "registry.json")
	registryData, _ := os.ReadFile(registryPath)
	var registry config.Registry
	json.Unmarshal(registryData, &registry)

	// Remove cmd2 from registry
	delete(registry.Wrappers, "cmd2")
	newData, _ := json.MarshalIndent(registry, "", "  ")
	os.WriteFile(registryPath, newData, 0644)

	// Step 5: Check status - cmd2 should not appear (not in registry)
	t.Log("Step 5: Status after registry modification...")
	output = env.MustRunRibbin(env.ProjectDir, "status")
	t.Logf("Status (registry modified): %s", output)

	// cmd1 and cmd3 should still appear
	env.AssertOutputContains(output, "cmd1")
	env.AssertOutputContains(output, "cmd3")

	// Step 6: Use 'ribbin find' to find orphaned sidecar
	t.Log("Step 6: Using 'ribbin find' to detect orphaned sidecar...")
	output, _ = env.RunRibbin(env.ProjectDir, "find", env.BinDir)
	t.Logf("Find output: %s", output)

	// Should find cmd2's orphaned sidecar
	env.AssertOutputContains(output, "cmd2")

	// Step 7: Clean up with unwrap --all --find
	t.Log("Step 7: Cleaning up with unwrap --all --find...")
	output = env.MustRunRibbin(env.ProjectDir, "unwrap", "--all", "--find")
	t.Logf("Unwrap output: %s", output)

	// Step 8: Verify all restored
	t.Log("Step 8: Verifying all restored...")
	for _, name := range bins {
		binPath := paths[name]
		sidecarPath := binPath + ".ribbin-original"

		env.AssertNotSymlink(binPath)
		env.AssertFileNotExists(sidecarPath)
	}

	t.Log("Status/find/status flow test completed!")

	_ = configPath
}

// TestOrphanedMetadataCleanup tests that metadata files are cleaned up with orphaned sidecars
// This test uses the real ribbin workflow to create a properly wrapped binary,
// then simulates an orphaned state by removing the registry entry.
func TestOrphanedMetadataCleanup(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	env.SetPathWithBinDir()
	env.InitGitRepo(env.ProjectDir)

	// Create test binary
	binPath := env.CreateMockBinaryWithOutput(env.BinDir, "orphan-cmd", "orphan-cmd output")

	// Build ribbin
	env.BuildRibbin("")

	// Create config and wrap properly
	env.CreateBlockConfig(env.ProjectDir, "orphan-cmd", "blocked", []string{binPath})
	env.GitAdd(env.ProjectDir, "ribbin.jsonc")
	env.GitCommit(env.ProjectDir, "Add config")

	// Wrap the binary properly
	env.MustRunRibbin(env.ProjectDir, "wrap")

	// Verify wrapped state
	sidecarPath := binPath + ".ribbin-original"
	metaPath := binPath + ".ribbin-meta"
	env.AssertFileExists(sidecarPath)
	env.AssertFileExists(metaPath)

	// Now simulate orphaned state by clearing the registry
	registryPath := filepath.Join(env.HomeDir, ".config", "ribbin", "registry.json")
	os.WriteFile(registryPath, []byte(`{"wrappers":{},"shell_activations":{},"config_activations":{},"global_active":false}`), 0644)

	// Clean up orphans using find
	output := env.MustRunRibbin(env.ProjectDir, "unwrap", "--all", "--find")
	t.Logf("Unwrap output: %s", output)

	// Verify all cleaned up
	env.AssertFileNotExists(sidecarPath)
	env.AssertFileNotExists(metaPath)
	env.AssertNotSymlink(binPath)
	env.AssertFileExists(binPath)

	t.Log("Orphaned metadata cleanup test completed!")
}
