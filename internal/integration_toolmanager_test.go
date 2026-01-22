package internal

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	_ "github.com/happycollision/ribbin/internal/testsafety"

	"github.com/happycollision/ribbin/internal/testutil"
	"github.com/happycollision/ribbin/internal/wrap"
)

// TestMiseCompatibility tests that ribbin works correctly with mise-style tool management.
// Mise installs binaries in ~/.local/share/mise/installs/<tool>/<version>/bin/
// and creates symlinks in ~/.local/share/mise/shims/ that point to the mise binary.
func TestMiseCompatibility(t *testing.T) {
	// Check if real mise is available
	misePath, err := exec.LookPath("mise")
	useMockMise := err != nil
	if useMockMise {
		t.Log("mise not found, using simulated mise environment")
	} else {
		t.Logf("Using real mise at: %s", misePath)
	}

	env := testutil.SetupIntegrationEnv(t)
	workDir := env.CreateDir("workdir")

	var miseShimsDir string
	var nodeShimPath string

	if useMockMise {
		// Create simulated mise environment
		miseInstallDir := filepath.Join(env.HomeDir, ".local", "share", "mise", "installs", "node", "20.0.0", "bin")
		miseShimsDir = filepath.Join(env.HomeDir, ".local", "share", "mise", "shims")

		for _, dir := range []string{miseInstallDir, miseShimsDir} {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatalf("failed to create dir %s: %v", dir, err)
			}
		}

		// Create mock "real" node
		realNodePath := filepath.Join(miseInstallDir, "node")
		if err := os.WriteFile(realNodePath, []byte("#!/bin/sh\necho \"MISE_NODE: v20.0.0\"\n"), 0755); err != nil {
			t.Fatalf("failed to create real node: %v", err)
		}

		// Create mock mise binary
		miseBinaryPath := filepath.Join(miseShimsDir, "mise")
		miseBinaryContent := `#!/bin/sh
exec "` + realNodePath + `" "$@"
`
		if err := os.WriteFile(miseBinaryPath, []byte(miseBinaryContent), 0755); err != nil {
			t.Fatalf("failed to create mise binary: %v", err)
		}

		// Create mise's node shim (symlink to mise)
		nodeShimPath = filepath.Join(miseShimsDir, "node")
		if err := os.Symlink(miseBinaryPath, nodeShimPath); err != nil {
			t.Fatalf("failed to create mise node shim: %v", err)
		}
	} else {
		// Use real mise - create mock tool since installing real tools is slow
		miseShimsDir = filepath.Join(env.HomeDir, ".local", "share", "mise", "shims")
		if err := os.MkdirAll(miseShimsDir, 0755); err != nil {
			t.Fatalf("failed to create shims dir: %v", err)
		}

		// Configure mise to use our home directory
		os.Setenv("MISE_DATA_DIR", filepath.Join(env.HomeDir, ".local", "share", "mise"))
		os.Setenv("MISE_CONFIG_DIR", filepath.Join(env.HomeDir, ".config", "mise"))
		os.Setenv("MISE_CACHE_DIR", filepath.Join(env.HomeDir, ".cache", "mise"))

		miseInstallDir := filepath.Join(env.HomeDir, ".local", "share", "mise", "installs", "dummy", "1.0.0", "bin")
		if err := os.MkdirAll(miseInstallDir, 0755); err != nil {
			t.Fatalf("failed to create install dir: %v", err)
		}

		// Create a dummy tool
		dummyPath := filepath.Join(miseInstallDir, "dummy-tool")
		if err := os.WriteFile(dummyPath, []byte("#!/bin/sh\necho \"MISE_DUMMY: v1.0.0\"\n"), 0755); err != nil {
			t.Fatalf("failed to create dummy tool: %v", err)
		}

		// Create mise-style shim
		nodeShimPath = filepath.Join(miseShimsDir, "dummy-tool")
		shimContent := `#!/bin/sh
exec "` + dummyPath + `" "$@"
`
		if err := os.WriteFile(nodeShimPath, []byte(shimContent), 0755); err != nil {
			t.Fatalf("failed to create shim: %v", err)
		}
	}

	os.Setenv("PATH", miseShimsDir+":"+env.GetOrigPath())

	// Verify shim works before ribbin
	cmd := exec.Command(nodeShimPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("mise shim should work before ribbin: %v\nOutput: %s", err, output)
	}
	t.Logf("Mise shim works before ribbin: %s", output)

	// Build ribbin
	ribbinPath := filepath.Join(miseShimsDir, "ribbin")
	buildCmd := exec.Command("go", "build", "-o", ribbinPath, "./cmd/ribbin")
	buildCmd.Dir = env.ModuleRoot
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build ribbin: %v\n%s", err, output)
	}
	env.RibbinPath = ribbinPath

	// Create ribbin.jsonc
	cmdName := filepath.Base(nodeShimPath)
	configPath := env.CreateBlockConfig(env.ProjectDir, cmdName, "Use something else", []string{nodeShimPath})

	// Install ribbin shim
	registry := env.NewRegistry()

	if err := wrap.Install(nodeShimPath, ribbinPath, registry, configPath); err != nil {
		t.Fatalf("failed to install shim: %v", err)
	}

	// Save registry
	env.SaveRegistry(registry)

	// Verify shim structure
	env.AssertSymlink(nodeShimPath, ribbinPath)

	sidecarPath := nodeShimPath + ".ribbin-original"
	env.AssertFileExists(sidecarPath)

	t.Log("Shim structure verified")

	// Test 1: From workDir (no ribbin.jsonc), command should passthrough
	os.Chdir(workDir)
	cmd = exec.Command(cmdName)
	cmd.Env = env.EnvironWithPath(miseShimsDir)
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Errorf("passthrough should work: %v\nOutput: %s", err, output)
	}
	t.Logf("Test 1 PASSED - Passthrough works: %s", output)

	// Test 2: RIBBIN_BYPASS=1 should passthrough
	cmd = exec.Command(cmdName)
	cmd.Env = append(env.EnvironWithPath(miseShimsDir), "RIBBIN_BYPASS=1")
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Errorf("bypass should work: %v\nOutput: %s", err, output)
	}
	t.Logf("Test 2 PASSED - Bypass works: %s", output)

	// Unshim and verify restoration
	if err := wrap.Uninstall(nodeShimPath, registry); err != nil {
		t.Fatalf("failed to uninstall shim: %v", err)
	}

	// Verify sidecar removed
	env.AssertFileNotExists(sidecarPath)

	// Verify original works
	cmd = exec.Command(nodeShimPath)
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Errorf("restored shim should work: %v\nOutput: %s", err, output)
	}
	t.Logf("Test 3 PASSED - Shim restored: %s", output)

	t.Log("Mise compatibility test completed successfully!")
}

// TestAsdfCompatibility tests that ribbin works correctly with asdf-style tool management.
// asdf installs binaries in ~/.asdf/installs/<tool>/<version>/bin/
// and creates shell script shims in ~/.asdf/shims/ (NOT symlinks, actual scripts).
func TestAsdfCompatibility(t *testing.T) {
	// Check if real asdf is available
	asdfPath, err := exec.LookPath("asdf")
	useMockAsdf := err != nil
	if useMockAsdf {
		t.Log("asdf not found, using simulated asdf environment")
	} else {
		t.Logf("Using real asdf at: %s", asdfPath)
	}

	env := testutil.SetupIntegrationEnv(t)
	workDir := env.CreateDir("workdir")

	var asdfShimsDir string
	var nodeShimPath string

	if useMockAsdf {
		// Create simulated asdf environment
		asdfInstallDir := filepath.Join(env.HomeDir, ".asdf", "installs", "nodejs", "20.0.0", "bin")
		asdfShimsDir = filepath.Join(env.HomeDir, ".asdf", "shims")

		for _, dir := range []string{asdfInstallDir, asdfShimsDir} {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatalf("failed to create dir %s: %v", dir, err)
			}
		}

		// Create mock "real" node
		realNodePath := filepath.Join(asdfInstallDir, "node")
		if err := os.WriteFile(realNodePath, []byte("#!/bin/sh\necho \"ASDF_NODE: v20.0.0\"\n"), 0755); err != nil {
			t.Fatalf("failed to create real node: %v", err)
		}

		// Create asdf-style shim (shell script, not symlink)
		nodeShimPath = filepath.Join(asdfShimsDir, "node")
		shimContent := `#!/bin/sh
exec "` + realNodePath + `" "$@"
`
		if err := os.WriteFile(nodeShimPath, []byte(shimContent), 0755); err != nil {
			t.Fatalf("failed to create asdf shim: %v", err)
		}
	} else {
		// Use real asdf - create mock tool
		asdfShimsDir = filepath.Join(env.HomeDir, ".asdf", "shims")
		if err := os.MkdirAll(asdfShimsDir, 0755); err != nil {
			t.Fatalf("failed to create shims dir: %v", err)
		}

		asdfInstallDir := filepath.Join(env.HomeDir, ".asdf", "installs", "dummy", "1.0.0", "bin")
		if err := os.MkdirAll(asdfInstallDir, 0755); err != nil {
			t.Fatalf("failed to create install dir: %v", err)
		}

		// Create a dummy tool
		dummyPath := filepath.Join(asdfInstallDir, "dummy-tool")
		if err := os.WriteFile(dummyPath, []byte("#!/bin/sh\necho \"ASDF_DUMMY: v1.0.0\"\n"), 0755); err != nil {
			t.Fatalf("failed to create dummy tool: %v", err)
		}

		// Create asdf-style shim
		nodeShimPath = filepath.Join(asdfShimsDir, "dummy-tool")
		shimContent := `#!/bin/sh
exec "` + dummyPath + `" "$@"
`
		if err := os.WriteFile(nodeShimPath, []byte(shimContent), 0755); err != nil {
			t.Fatalf("failed to create shim: %v", err)
		}
	}

	os.Setenv("PATH", asdfShimsDir+":"+env.GetOrigPath())

	// Verify shim works before ribbin
	cmd := exec.Command(nodeShimPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("asdf shim should work before ribbin: %v\nOutput: %s", err, output)
	}
	t.Logf("asdf shim works before ribbin: %s", output)

	// Build ribbin
	ribbinPath := filepath.Join(asdfShimsDir, "ribbin")
	buildCmd := exec.Command("go", "build", "-o", ribbinPath, "./cmd/ribbin")
	buildCmd.Dir = env.ModuleRoot
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build ribbin: %v\n%s", err, output)
	}
	env.RibbinPath = ribbinPath

	// Create ribbin.jsonc
	cmdName := filepath.Base(nodeShimPath)
	configPath := env.CreateBlockConfig(env.ProjectDir, cmdName, "Use something else", []string{nodeShimPath})

	// Install ribbin shim
	registry := env.NewRegistry()

	if err := wrap.Install(nodeShimPath, ribbinPath, registry, configPath); err != nil {
		t.Fatalf("failed to install shim: %v", err)
	}

	// Save registry
	env.SaveRegistry(registry)

	// Verify shim structure (should be symlink now)
	env.AssertSymlink(nodeShimPath, ribbinPath)

	sidecarPath := nodeShimPath + ".ribbin-original"
	env.AssertFileExists(sidecarPath)

	t.Log("Shim structure verified")

	// Test: From workDir (no ribbin.jsonc), command should passthrough
	os.Chdir(workDir)
	cmd = exec.Command(cmdName)
	cmd.Env = env.EnvironWithPath(asdfShimsDir)
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Errorf("passthrough should work: %v\nOutput: %s", err, output)
	}
	t.Logf("Passthrough works: %s", output)

	// Unshim and verify restoration
	if err := wrap.Uninstall(nodeShimPath, registry); err != nil {
		t.Fatalf("failed to uninstall shim: %v", err)
	}

	// Verify sidecar removed
	env.AssertFileNotExists(sidecarPath)

	// Verify original works and is NOT a symlink (asdf shims are scripts)
	env.AssertNotSymlink(nodeShimPath)

	cmd = exec.Command(nodeShimPath)
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Errorf("restored shim should work: %v\nOutput: %s", err, output)
	}
	t.Logf("Shim restored: %s", output)

	t.Log("asdf compatibility test completed successfully!")
}

// TestMiseWithActivation tests mise compatibility with ribbin activation
func TestMiseWithActivation(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	env.SetPathWithBinDir()

	// Create simulated mise environment
	miseInstallDir := filepath.Join(env.HomeDir, ".local", "share", "mise", "installs", "node", "20.0.0", "bin")
	miseShimsDir := filepath.Join(env.HomeDir, ".local", "share", "mise", "shims")

	for _, dir := range []string{miseInstallDir, miseShimsDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	// Create mock node
	realNodePath := filepath.Join(miseInstallDir, "node")
	if err := os.WriteFile(realNodePath, []byte("#!/bin/sh\necho \"MISE_NODE: v20.0.0\"\n"), 0755); err != nil {
		t.Fatalf("failed to create real node: %v", err)
	}

	// Create mise's node shim
	nodeShimPath := filepath.Join(miseShimsDir, "node")
	shimContent := `#!/bin/sh
exec "` + realNodePath + `" "$@"
`
	if err := os.WriteFile(nodeShimPath, []byte(shimContent), 0755); err != nil {
		t.Fatalf("failed to create shim: %v", err)
	}

	// Build ribbin into mise shims dir
	ribbinPath := filepath.Join(miseShimsDir, "ribbin")
	buildCmd := exec.Command("go", "build", "-o", ribbinPath, "./cmd/ribbin")
	buildCmd.Dir = env.ModuleRoot
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build ribbin: %v\n%s", err, output)
	}
	env.RibbinPath = ribbinPath

	os.Setenv("PATH", miseShimsDir+":"+env.BinDir+":"+env.GetOrigPath())

	// Init git repo
	env.InitGitRepo(env.ProjectDir)

	// Create config
	configContent := fmt.Sprintf(`{
  "wrappers": {
    "node": {
      "action": "block",
      "message": "Use nvm instead",
      "paths": ["%s"]
    }
  }
}`, nodeShimPath)
	env.CreateConfig(env.ProjectDir, configContent)
	env.GitAdd(env.ProjectDir, "ribbin.jsonc")
	env.GitCommit(env.ProjectDir, "Add config")

	// Wrap
	output := env.MustRunRibbin(env.ProjectDir, "wrap", "--confirm-system-dir")
	t.Logf("Wrap output: %s", output)

	// Activate globally
	output = env.MustRunRibbin(env.ProjectDir, "activate", "--global")
	t.Logf("Activate output: %s", output)

	// Verify shim
	env.AssertSymlink(nodeShimPath, ribbinPath)

	// Unwrap
	output = env.MustRunRibbin(env.ProjectDir, "unwrap")
	t.Logf("Unwrap output: %s", output)

	// Verify restored
	env.AssertNotSymlink(nodeShimPath)

	t.Log("Mise with activation test completed!")
}

// TestMiseManagedBinaryWrapping tests wrapping a binary managed by mise (symlink-based shims).
func TestMiseManagedBinaryWrapping(t *testing.T) {
	// Check if real mise is available
	systemMisePath, err := exec.LookPath("mise")
	if err != nil {
		t.Skip("mise not found, skipping real mise test")
	}
	t.Logf("Found mise at: %s", systemMisePath)

	env := testutil.SetupIntegrationEnv(t)
	env.InitGitRepo(env.ProjectDir)

	// Configure mise to use our test directories
	os.Setenv("MISE_DATA_DIR", filepath.Join(env.HomeDir, ".local", "share", "mise"))
	os.Setenv("MISE_CONFIG_DIR", filepath.Join(env.HomeDir, ".config", "mise"))
	os.Setenv("MISE_CACHE_DIR", filepath.Join(env.HomeDir, ".cache", "mise"))

	miseShimsDir := filepath.Join(env.HomeDir, ".local", "share", "mise", "shims")
	if err := os.MkdirAll(miseShimsDir, 0755); err != nil {
		t.Fatalf("failed to create shims dir: %v", err)
	}

	// Create a mock shfmt tool
	miseInstallDir := filepath.Join(env.HomeDir, ".local", "share", "mise", "installs", "shfmt", "3.0.0", "bin")
	if err := os.MkdirAll(miseInstallDir, 0755); err != nil {
		t.Fatalf("failed to create install dir: %v", err)
	}

	shfmtPath := filepath.Join(miseInstallDir, "shfmt")
	if err := os.WriteFile(shfmtPath, []byte("#!/bin/sh\necho \"shfmt v3.0.0\"\n"), 0755); err != nil {
		t.Fatalf("failed to create shfmt: %v", err)
	}

	// Create mise-style shim
	shimPath := filepath.Join(miseShimsDir, "shfmt")
	shimContent := `#!/bin/sh
exec "` + shfmtPath + `" "$@"
`
	if err := os.WriteFile(shimPath, []byte(shimContent), 0755); err != nil {
		t.Fatalf("failed to create shim: %v", err)
	}

	os.Setenv("PATH", miseShimsDir+":"+env.BinDir+":"+env.GetOrigPath())

	// Build ribbin
	env.BuildRibbin("")

	// Create config
	env.CreateBlockConfig(env.ProjectDir, "shfmt", "Use project formatter", []string{shimPath})
	env.GitAdd(env.ProjectDir, "ribbin.jsonc")
	env.GitCommit(env.ProjectDir, "Add config")

	// Wrap
	output := env.MustRunRibbin(env.ProjectDir, "wrap", "--confirm-system-dir")
	t.Logf("Wrap output: %s", output)

	// Activate
	output = env.MustRunRibbin(env.ProjectDir, "activate", "--global")
	t.Logf("Activate output: %s", output)

	// Verify structure
	env.AssertSymlink(shimPath, env.RibbinPath)

	// Unwrap
	output = env.MustRunRibbin(env.ProjectDir, "unwrap")
	t.Logf("Unwrap output: %s", output)

	// Verify restored
	env.AssertNotSymlink(shimPath)

	t.Log("Mise-managed binary wrapping test completed!")
}

// TestAsdfManagedBinaryWrapping tests wrapping a binary managed by asdf (script-based shims).
func TestAsdfManagedBinaryWrapping(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	env.InitGitRepo(env.ProjectDir)

	// Create simulated asdf environment
	asdfInstallDir := filepath.Join(env.HomeDir, ".asdf", "installs", "shfmt", "3.0.0", "bin")
	asdfShimsDir := filepath.Join(env.HomeDir, ".asdf", "shims")

	for _, dir := range []string{asdfInstallDir, asdfShimsDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	// Create mock shfmt
	shfmtPath := filepath.Join(asdfInstallDir, "shfmt")
	if err := os.WriteFile(shfmtPath, []byte("#!/bin/sh\necho \"shfmt v3.0.0\"\n"), 0755); err != nil {
		t.Fatalf("failed to create shfmt: %v", err)
	}

	// Create asdf-style shim (script, not symlink)
	shimPath := filepath.Join(asdfShimsDir, "shfmt")
	shimContent := `#!/bin/sh
exec "` + shfmtPath + `" "$@"
`
	if err := os.WriteFile(shimPath, []byte(shimContent), 0755); err != nil {
		t.Fatalf("failed to create shim: %v", err)
	}

	os.Setenv("PATH", asdfShimsDir+":"+env.BinDir+":"+env.GetOrigPath())

	// Build ribbin
	env.BuildRibbin("")

	// Create config
	env.CreateBlockConfig(env.ProjectDir, "shfmt", "Use project formatter", []string{shimPath})
	env.GitAdd(env.ProjectDir, "ribbin.jsonc")
	env.GitCommit(env.ProjectDir, "Add config")

	// Wrap
	output := env.MustRunRibbin(env.ProjectDir, "wrap", "--confirm-system-dir")
	t.Logf("Wrap output: %s", output)

	// Activate
	output = env.MustRunRibbin(env.ProjectDir, "activate", "--global")
	t.Logf("Activate output: %s", output)

	// Verify structure
	env.AssertSymlink(shimPath, env.RibbinPath)

	// Unwrap
	output = env.MustRunRibbin(env.ProjectDir, "unwrap")
	t.Logf("Unwrap output: %s", output)

	// Verify restored - should be a regular file (not symlink)
	env.AssertNotSymlink(shimPath)

	t.Log("asdf-managed binary wrapping test completed!")
}
