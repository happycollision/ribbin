package internal

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	_ "github.com/happycollision/ribbin/internal/testsafety"

	"github.com/happycollision/ribbin/internal/config"
	"github.com/happycollision/ribbin/internal/testutil"
	"github.com/happycollision/ribbin/internal/wrap"
)

// TestRedirectAction tests the redirect action functionality end-to-end.
func TestRedirectAction(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	env.SetPathWithBinDir()

	// Build ribbin binary
	env.BuildRibbin("")

	// Copy test fixtures from testdata/projects/redirect/ to project dir
	fixtureDir := filepath.Join(env.ModuleRoot, "testdata", "projects", "redirect")

	// Copy ribbin.jsonc
	fixtureConfigPath := filepath.Join(fixtureDir, "ribbin.jsonc")
	fixtureConfig, err := os.ReadFile(fixtureConfigPath)
	if err != nil {
		t.Fatalf("failed to read fixture config: %v", err)
	}
	configPath := filepath.Join(env.ProjectDir, "ribbin.jsonc")
	if err := os.WriteFile(configPath, fixtureConfig, 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Copy redirect script
	scriptsDir := filepath.Join(env.ProjectDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatalf("failed to create scripts dir: %v", err)
	}
	fixtureScriptPath := filepath.Join(fixtureDir, "scripts", "test-redirect.sh")
	fixtureScript, err := os.ReadFile(fixtureScriptPath)
	if err != nil {
		t.Fatalf("failed to read fixture script: %v", err)
	}
	scriptPath := filepath.Join(scriptsDir, "test-redirect.sh")
	if err := os.WriteFile(scriptPath, fixtureScript, 0755); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	// Create the original echo command
	echoCmdPath := env.CreateMockBinaryWithOutput(env.BinDir, "echo", "ORIGINAL_ECHO: $@")

	// Install shim
	registry := env.NewRegistry()
	registry.GlobalActive = true

	if err := wrap.Install(echoCmdPath, env.RibbinPath, registry, configPath); err != nil {
		t.Fatalf("failed to install shim: %v", err)
	}

	// Save registry
	env.SaveRegistry(registry)

	// Change to project directory (where ribbin.jsonc is)
	env.ChdirProject()

	// Execute the shimmed echo command with arguments
	cmd := exec.Command("echo", "arg1", "arg2", "arg3")
	cmd.Env = env.Environ()
	output, err := cmd.CombinedOutput()

	// Command should succeed (exit 0)
	if err != nil {
		t.Errorf("redirect script should exit 0: %v\nOutput: %s", err, output)
	}

	outputStr := string(output)
	t.Logf("Redirect output: %s", outputStr)

	// Verify output contains expected markers
	env.AssertOutputContains(outputStr, "REDIRECT_CALLED=true")

	// Verify environment variables are set
	env.AssertOutputContains(outputStr, "RIBBIN_ORIGINAL_BIN=")
	env.AssertOutputContains(outputStr, "RIBBIN_COMMAND=")
	env.AssertOutputContains(outputStr, "RIBBIN_CONFIG=")
	env.AssertOutputContains(outputStr, "RIBBIN_ACTION=redirect")

	// Verify arguments are forwarded
	env.AssertOutputContains(outputStr, "ARGS=arg1 arg2 arg3")

	// Test with RIBBIN_BYPASS - should execute original echo
	cmd = exec.Command("echo", "bypass-test")
	cmd.Env = env.EnvironWith("RIBBIN_BYPASS=1")
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Errorf("bypass should work: %v\nOutput: %s", err, output)
	}
	env.AssertOutputContains(string(output), "ORIGINAL_ECHO")
	t.Logf("Bypass test passed: %s", output)

	t.Log("Redirect action test completed successfully!")
}

// TestShimPathResolution tests path resolution for shimmed commands
func TestShimPathResolution(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	env.SetPathWithBinDir()

	// Create test binary in a nested path
	nestedBinDir := env.CreateDir("nested/bin")
	testBinaryPath := env.CreateMockBinaryWithOutput(nestedBinDir, "nested-cmd", "nested command output")

	// Create fake ribbin binary
	ribbinPath := filepath.Join(env.BinDir, "ribbin")
	ribbinContent := `#!/bin/sh
echo "ribbin intercepted: $0"
exit 0
`
	if err := os.WriteFile(ribbinPath, []byte(ribbinContent), 0755); err != nil {
		t.Fatalf("failed to create ribbin: %v", err)
	}

	// Create config pointing to the nested binary
	configPath := env.CreateBlockConfig(env.ProjectDir, "nested-cmd", "Use proper-nested-cmd", []string{testBinaryPath})

	// Install shim
	registry := env.NewRegistry()
	if err := wrap.Install(testBinaryPath, ribbinPath, registry, configPath); err != nil {
		t.Fatalf("failed to install shim: %v", err)
	}

	// Verify the shim was installed at the correct path
	env.AssertSymlink(testBinaryPath, ribbinPath)

	// Verify sidecar exists
	sidecarPath := testBinaryPath + ".ribbin-original"
	env.AssertFileExists(sidecarPath)

	// Uninstall and verify
	if err := wrap.Uninstall(testBinaryPath, registry); err != nil {
		t.Fatalf("failed to uninstall: %v", err)
	}

	env.AssertNotSymlink(testBinaryPath)
	env.AssertFileNotExists(sidecarPath)

	t.Log("Shim path resolution test completed!")
}

// TestSymlinkTargetResolution tests that symlink targets are resolved correctly
func TestSymlinkTargetResolution(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)

	// Create a real binary
	realBinDir := env.CreateDir("real-bin")
	realBinaryPath := filepath.Join(realBinDir, "real-cmd")
	if err := os.WriteFile(realBinaryPath, []byte("#!/bin/sh\necho real\n"), 0755); err != nil {
		t.Fatalf("failed to create real binary: %v", err)
	}

	// Create a symlink to the real binary
	linkPath := filepath.Join(env.BinDir, "linked-cmd")
	if err := os.Symlink(realBinaryPath, linkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	// Verify the symlink points to the correct target
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("failed to read symlink: %v", err)
	}
	if target != realBinaryPath {
		t.Errorf("expected symlink to point to %s, got %s", realBinaryPath, target)
	}

	// Create ribbin binary
	ribbinPath := filepath.Join(env.BinDir, "ribbin")
	if err := os.WriteFile(ribbinPath, []byte("#!/bin/sh\necho ribbin\n"), 0755); err != nil {
		t.Fatalf("failed to create ribbin: %v", err)
	}

	// Create config
	configPath := env.CreateBlockConfig(env.ProjectDir, "linked-cmd", "blocked", []string{linkPath})

	// Install shim on the symlink
	registry := env.NewRegistry()
	if err := wrap.Install(linkPath, ribbinPath, registry, configPath); err != nil {
		t.Fatalf("failed to install shim: %v", err)
	}

	// Verify the symlink was replaced with a symlink to ribbin
	newTarget, _ := os.Readlink(linkPath)
	if newTarget != ribbinPath {
		t.Errorf("expected shimmed link to point to ribbin, got %s", newTarget)
	}

	// Verify sidecar contains the original symlink target (or the script)
	sidecarPath := linkPath + ".ribbin-original"
	env.AssertFileExists(sidecarPath)

	// Uninstall
	if err := wrap.Uninstall(linkPath, registry); err != nil {
		t.Fatalf("failed to uninstall: %v", err)
	}

	// Verify restoration
	env.AssertFileNotExists(sidecarPath)

	t.Log("Symlink target resolution test completed!")
}

// TestConfigFromDifferentDirectory tests running shimmed command from different directories
func TestConfigFromDifferentDirectory(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	env.SetPathWithBinDir()

	// Create directories
	configDir := env.CreateDir("config-project")
	workDir := env.CreateDir("work-area")

	// Create test binary
	testBinaryPath := env.CreateMockBinaryWithOutput(env.BinDir, "test-cmd", "test-cmd output")

	// Create config in configDir
	configPath := env.CreateBlockConfig(configDir, "test-cmd", "blocked in config-project", []string{testBinaryPath})

	// Build ribbin
	env.BuildRibbin("")

	// Initialize git repo in configDir
	env.InitGitRepo(configDir)
	env.GitAdd(configDir, "ribbin.jsonc")
	env.GitCommit(configDir, "Add config")

	// Wrap
	env.MustRunRibbin(configDir, "wrap")
	env.MustRunRibbin(configDir, "activate", "--global")

	// Test from configDir - should be blocked
	env.Chdir(configDir)
	cmd := exec.Command(testBinaryPath)
	cmd.Env = env.Environ()
	output, err := cmd.CombinedOutput()

	if err == nil {
		t.Logf("Command from configDir: %s", output)
	} else {
		t.Logf("Command blocked from configDir (expected): %s", output)
	}

	// Test from workDir - should passthrough (no config there)
	env.Chdir(workDir)
	cmd = exec.Command(testBinaryPath)
	cmd.Env = env.Environ()
	output, err = cmd.CombinedOutput()

	if err != nil {
		t.Errorf("command should passthrough from workDir: %v\n%s", err, output)
	} else {
		env.AssertOutputContains(string(output), "test-cmd output")
		t.Logf("Command passthrough from workDir: %s", output)
	}

	// Cleanup
	env.MustRunRibbin(configDir, "unwrap", configPath)

	t.Log("Config from different directory test completed!")
}

// TestMultipleConfigsInHierarchy tests config discovery with multiple configs
func TestMultipleConfigsInHierarchy(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)

	// Create hierarchy: /parent/child/grandchild
	parentDir := env.CreateDir("parent")
	childDir := env.CreateDir("parent/child")
	grandchildDir := env.CreateDir("parent/child/grandchild")

	// Create config in parent
	parentConfig := `{
  "wrappers": {
    "parent-cmd": {
      "action": "block",
      "message": "parent blocked"
    }
  }
}`
	env.CreateConfig(parentDir, parentConfig)

	// Create config in child (should override)
	childConfig := `{
  "wrappers": {
    "child-cmd": {
      "action": "block",
      "message": "child blocked"
    }
  }
}`
	env.CreateConfig(childDir, childConfig)

	// Test config discovery from grandchild - should find child config first
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(grandchildDir)

	configPath, err := config.FindProjectConfig()
	if err != nil {
		t.Fatalf("failed to find config: %v", err)
	}

	expectedPath := filepath.Join(childDir, "ribbin.jsonc")
	if configPath != expectedPath {
		t.Errorf("expected config at %s, got %s", expectedPath, configPath)
	}

	// Load and verify it's the child config
	cfg, err := config.LoadProjectConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if _, ok := cfg.Wrappers["child-cmd"]; !ok {
		t.Error("expected child-cmd in config")
	}
	if _, ok := cfg.Wrappers["parent-cmd"]; ok {
		t.Error("should not have parent-cmd (child config doesn't extend)")
	}

	t.Log("Multiple configs in hierarchy test completed!")
}
