package internal

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/happycollision/ribbin/internal/testsafety"

	"github.com/happycollision/ribbin/internal/config"
	"github.com/happycollision/ribbin/internal/testutil"
	"github.com/happycollision/ribbin/internal/wrap"
)

// TestFullShimCycle tests the complete shim install/activate/block/uninstall workflow
func TestFullShimCycle(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	env.SetPathWithBinDir()
	env.ChdirProject()

	// Step 1: Create a test binary
	testBinaryPath := env.CreateMockBinaryWithOutput(env.BinDir, "test-cmd", "original test-cmd executed with args: $@")
	t.Log("Step 1: Created test binary")

	// Create a fake ribbin binary (in real use this would be the actual ribbin binary)
	ribbinPath := filepath.Join(env.BinDir, "ribbin")
	ribbinContent := `#!/bin/sh
echo "ribbin shim intercepted: $0 $@"
exit 1
`
	if err := os.WriteFile(ribbinPath, []byte(ribbinContent), 0755); err != nil {
		t.Fatalf("failed to create ribbin binary: %v", err)
	}

	// Step 2: Create ribbin.jsonc
	configPath := env.CreateBlockConfig(env.ProjectDir, "test-cmd", "Use 'proper-cmd' instead", []string{testBinaryPath})
	t.Log("Step 2: Created ribbin.jsonc")

	// Step 3: Install shim
	registry := env.NewRegistry()

	if err := wrap.Install(testBinaryPath, ribbinPath, registry, configPath); err != nil {
		t.Fatalf("failed to install shim: %v", err)
	}
	t.Log("Step 3: Installed shim")

	// Verify symlink was created
	env.AssertSymlink(testBinaryPath, ribbinPath)

	// Verify sidecar exists
	sidecarPath := testBinaryPath + ".ribbin-original"
	env.AssertFileExists(sidecarPath)

	// Verify registry was updated
	if _, exists := registry.Wrappers["test-cmd"]; !exists {
		t.Error("registry should contain test-cmd entry")
	}

	// Save registry
	env.SaveRegistry(registry)
	t.Log("Step 4: Saved registry")

	// Step 5: Test running shimmed command (should execute original via symlink)
	output, _ := env.RunCmd(env.ProjectDir, testBinaryPath, "arg1", "arg2")
	t.Logf("Shimmed command output: %s", output)
	t.Log("Step 5: Tested shimmed command execution")

	// Step 6: Uninstall shim
	if err := wrap.Uninstall(testBinaryPath, registry); err != nil {
		t.Fatalf("failed to uninstall shim: %v", err)
	}
	t.Log("Step 6: Uninstalled shim")

	// Verify original is restored
	env.AssertNotSymlink(testBinaryPath)

	// Verify sidecar is removed
	env.AssertFileNotExists(sidecarPath)

	// Verify registry was updated
	if _, exists := registry.Wrappers["test-cmd"]; exists {
		t.Error("registry should not contain test-cmd entry after uninstall")
	}

	// Step 7: Test running restored command
	output, err := env.RunCmd(env.ProjectDir, testBinaryPath, "arg1", "arg2")
	if err != nil {
		t.Fatalf("restored command should run successfully: %v, output: %s", err, output)
	}
	t.Logf("Restored command output: %s", output)
	t.Log("Step 7: Verified original binary restored and executable")

	t.Log("Full shim cycle completed successfully!")
}

// TestRegistryPersistence tests registry save/load cycle
func TestRegistryPersistence(t *testing.T) {
	_ = testutil.SetupIntegrationEnv(t) // Sets up HOME for registry path

	// Create and save registry
	registry := &config.Registry{
		Wrappers: map[string]config.WrapperEntry{
			"cat":  {Original: "/usr/bin/cat", Config: "/project/ribbin.jsonc"},
			"node": {Original: "/usr/local/bin/node", Config: "/other/ribbin.jsonc"},
		},
		ShellActivations:  make(map[int]config.ShellActivationEntry),
		ConfigActivations: make(map[string]config.ConfigActivationEntry),
		GlobalActive:      true,
	}

	if err := config.SaveRegistry(registry); err != nil {
		t.Fatalf("SaveRegistry error: %v", err)
	}

	// Load and verify
	loaded, err := config.LoadRegistry()
	if err != nil {
		t.Fatalf("LoadRegistry error: %v", err)
	}

	if !loaded.GlobalActive {
		t.Error("GlobalOn should be true")
	}
	if len(loaded.Wrappers) != 2 {
		t.Errorf("expected 2 shims, got %d", len(loaded.Wrappers))
	}
	if loaded.Wrappers["cat"].Original != "/usr/bin/cat" {
		t.Error("cat shim Original mismatch")
	}
}

// TestUnwrapInconsistentState tests unwrapping when sidecar exists but binary
// is not a symlink (e.g., the tool was reinstalled after wrapping).
func TestUnwrapInconsistentState(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	env.SetPathWithBinDir()

	// Initialize git repo (required for ribbin)
	env.InitGitRepo(env.ProjectDir)

	// Step 1: Create mock binary
	tscPath := filepath.Join(env.BinDir, "tsc")
	originalContent := "#!/bin/sh\necho 'tsc: original version'\nexit 0\n"
	if err := os.WriteFile(tscPath, []byte(originalContent), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}
	t.Log("Step 1: Created mock tsc binary")

	// Step 2: Build ribbin binary
	env.BuildRibbin("")
	t.Log("Step 2: Built ribbin binary")

	// Step 3: Create ribbin.jsonc
	env.CreateBlockConfig(env.ProjectDir, "tsc", "Use project script instead", []string{tscPath})
	env.GitAdd(env.ProjectDir, "ribbin.jsonc")
	env.GitCommit(env.ProjectDir, "Add config")
	t.Log("Step 3: Created ribbin.jsonc")

	// Step 4: Wrap the binary
	t.Log("Step 4: Running 'ribbin wrap'...")
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

	// Step 5: Simulate the tool being reinstalled - replace symlink with regular file
	t.Log("Step 5: Simulating tool reinstall (replacing symlink with regular file)...")
	if err := os.Remove(tscPath); err != nil {
		t.Fatalf("failed to remove symlink: %v", err)
	}

	reinstalledContent := "#!/bin/sh\necho 'tsc: NEW reinstalled version'\nexit 0\n"
	if err := os.WriteFile(tscPath, []byte(reinstalledContent), 0755); err != nil {
		t.Fatalf("failed to write reinstalled binary: %v", err)
	}

	// Step 6: Verify inconsistent state
	t.Log("Step 6: Verifying inconsistent state...")
	env.AssertNotSymlink(tscPath)
	env.AssertFileExists(sidecarPath)

	// Step 7: Run unwrap - should detect inconsistent state and clean up
	t.Log("Step 7: Running 'ribbin unwrap --all'...")
	output, _ = env.RunRibbin(env.ProjectDir, "unwrap", "--all")
	t.Logf("Unwrap output: %s", output)

	// Step 8: Verify cleanup happened
	t.Log("Step 8: Verifying cleanup...")

	// Sidecar should be removed
	env.AssertFileNotExists(sidecarPath)

	// Metadata should be removed
	metaPath := tscPath + ".ribbin-meta"
	env.AssertFileNotExists(metaPath)

	// Current binary should still be there (the reinstalled version)
	env.AssertFileExists(tscPath)

	// Verify it's the reinstalled version
	content, err := os.ReadFile(tscPath)
	if err != nil {
		t.Fatalf("failed to read tsc: %v", err)
	}
	if !testutil.Contains(string(content), "NEW reinstalled version") {
		t.Error("expected current binary to be the reinstalled version")
	}

	// Registry should be cleaned
	registry := env.LoadRegistry()
	if _, exists := registry.Wrappers["tsc"]; exists {
		t.Error("expected tsc to be removed from registry after unwrap")
	}

	t.Log("Unwrap inconsistent state test completed successfully!")
}
