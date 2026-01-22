package internal

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/happycollision/ribbin/internal/testsafety"

	"github.com/happycollision/ribbin/internal/testutil"
)

// TestNodeModulesTscWrappingNpm tests wrapping node_modules/.bin/tsc installed via npm.
func TestNodeModulesTscWrappingNpm(t *testing.T) {
	if _, err := exec.LookPath("npm"); err != nil {
		t.Skip("npm not found, skipping test")
	}

	testNodeModulesTscWrapping(t, "npm")
}

// TestNodeModulesTscWrappingPnpm tests wrapping node_modules/.bin/tsc installed via pnpm.
func TestNodeModulesTscWrappingPnpm(t *testing.T) {
	if _, err := exec.LookPath("pnpm"); err != nil {
		t.Skip("pnpm not found, skipping test")
	}

	testNodeModulesTscWrapping(t, "pnpm")
}

// testNodeModulesTscWrapping is the shared test implementation for both npm and pnpm.
func testNodeModulesTscWrapping(t *testing.T, packageManager string) {
	env := testutil.SetupIntegrationEnv(t)

	parentDir := env.CreateDir("parent")
	projectDir := filepath.Join(parentDir, "project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	// Build ribbin
	env.BuildRibbin("")

	// Create package.json with TypeScript as a dev dependency
	packageJSON := `{
  "name": "test-project",
  "version": "1.0.0",
  "devDependencies": {
    "typescript": "^5.0.0"
  }
}`
	if err := os.WriteFile(filepath.Join(projectDir, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatalf("failed to write package.json: %v", err)
	}

	// Install dependencies
	t.Logf("Installing TypeScript with %s...", packageManager)
	installCmd := exec.Command(packageManager, "install")
	installCmd.Dir = projectDir
	installCmd.Env = append(os.Environ(), "HOME="+env.HomeDir, "CI=true")
	if output, err := installCmd.CombinedOutput(); err != nil {
		t.Fatalf("%s install failed: %v\n%s", packageManager, err, output)
	}
	t.Logf("%s install completed", packageManager)

	// Verify tsc exists in node_modules/.bin/
	tscPath := filepath.Join(projectDir, "node_modules", ".bin", "tsc")
	env.AssertFileExists(tscPath)

	// Log what type of binary tsc is
	tscInfo, _ := os.Lstat(tscPath)
	isSymlink := tscInfo.Mode()&os.ModeSymlink != 0
	t.Logf("tsc is symlink: %v", isSymlink)

	// Verify tsc runs before shimming
	t.Log("Verifying tsc works before shimming...")
	tscCmd := exec.Command(tscPath, "--version")
	tscCmd.Env = env.Environ()
	output, err := tscCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("tsc --version failed before shimming: %v\n%s", err, output)
	}
	t.Logf("tsc version: %s", output)

	// Create ribbin.jsonc in PARENT directory (testing parent dir config)
	configContent := fmt.Sprintf(`{
  "wrappers": {
    "tsc": {
      "action": "block",
      "message": "Use 'pnpm run typecheck' instead of tsc directly",
      "paths": ["%s"]
    }
  }
}`, tscPath)
	configPath := filepath.Join(parentDir, "ribbin.jsonc")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write ribbin.jsonc: %v", err)
	}

	// Use CLI to wrap tsc
	t.Log("Running ribbin wrap...")
	wrapCmd := exec.Command(env.RibbinPath, "wrap", "--confirm-system-dir", configPath)
	wrapCmd.Dir = parentDir
	wrapCmd.Env = env.Environ()
	output, err = wrapCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ribbin wrap failed: %v\n%s", err, output)
	}
	t.Logf("ribbin wrap output: %s", output)

	// Activate globally
	t.Log("Running ribbin activate --global...")
	activateCmd := exec.Command(env.RibbinPath, "activate", "--global")
	activateCmd.Dir = parentDir
	activateCmd.Env = env.Environ()
	output, err = activateCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ribbin activate --global failed: %v\n%s", err, output)
	}
	t.Logf("ribbin activate output: %s", output)

	// Verify shim structure
	env.AssertSymlink(tscPath, env.RibbinPath)

	// Verify sidecar exists
	sidecarPath := tscPath + ".ribbin-original"
	env.AssertFileExists(sidecarPath)
	t.Logf("Sidecar exists: %s", sidecarPath)

	// Test 1: From project directory, tsc should be BLOCKED
	t.Log("Test 1: tsc should be blocked from project directory")
	os.Chdir(projectDir)
	tscCmd = exec.Command(tscPath, "--version")
	tscCmd.Env = env.Environ()
	output, err = tscCmd.CombinedOutput()
	if err == nil {
		t.Errorf("tsc should be blocked, but succeeded: %s", output)
	} else {
		t.Logf("Test 1 PASSED - tsc blocked: %s", output)
	}

	// Test 2: With RIBBIN_BYPASS=1, tsc should work
	t.Log("Test 2: tsc should work with RIBBIN_BYPASS=1")
	tscCmd = exec.Command(tscPath, "--version")
	tscCmd.Env = env.EnvironWith("RIBBIN_BYPASS=1")
	output, err = tscCmd.CombinedOutput()
	if err != nil {
		t.Errorf("tsc with bypass should work: %v\n%s", err, output)
	} else {
		env.AssertOutputContains(string(output), "Version")
		t.Logf("Test 2 PASSED - tsc with bypass: %s", output)
	}

	// Test 3: Run tsc by name (via PATH) from project directory
	t.Log("Test 3: tsc by name should be blocked")
	nodeModulesBin := filepath.Join(projectDir, "node_modules", ".bin")
	tscCmd = exec.Command("tsc", "--version")
	tscCmd.Env = env.EnvironWithPath(nodeModulesBin + ":" + env.BinDir)
	output, err = tscCmd.CombinedOutput()
	if err == nil {
		t.Errorf("tsc by name should be blocked, but succeeded: %s", output)
	} else {
		t.Logf("Test 3 PASSED - tsc by name blocked: %s", output)
	}

	// Use CLI to unwrap
	t.Log("Running ribbin unwrap...")
	unwrapCmd := exec.Command(env.RibbinPath, "unwrap", configPath)
	unwrapCmd.Dir = parentDir
	unwrapCmd.Env = env.Environ()
	output, err = unwrapCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ribbin unwrap failed: %v\n%s", err, output)
	}
	t.Logf("ribbin unwrap output: %s", output)

	// Verify tsc works after unshimming
	t.Log("Verifying tsc works after unshimming...")
	tscCmd = exec.Command(tscPath, "--version")
	tscCmd.Env = env.Environ()
	output, err = tscCmd.CombinedOutput()
	if err != nil {
		t.Errorf("tsc should work after unshimming: %v\n%s", err, output)
	} else {
		t.Logf("tsc restored and working: %s", output)
	}

	// Verify sidecar is removed
	env.AssertFileNotExists(sidecarPath)

	t.Logf("%s node_modules test completed!", packageManager)
}

// TestScopeWrappersWithRealPnpm tests that scope wrappers work with real pnpm and TypeScript.
func TestScopeWrappersWithRealPnpm(t *testing.T) {
	if _, err := exec.LookPath("pnpm"); err != nil {
		t.Skip("pnpm not available, skipping test")
	}

	env := testutil.SetupIntegrationEnv(t)

	projectDir := env.CreateDir("project")
	frontendDir := filepath.Join(projectDir, "frontend")
	if err := os.MkdirAll(frontendDir, 0755); err != nil {
		t.Fatalf("failed to create frontend dir: %v", err)
	}

	// Initialize git repo
	env.InitGitRepo(projectDir)

	// Create package.json with TypeScript IN THE FRONTEND DIRECTORY
	packageJSON := `{
  "name": "frontend",
  "version": "1.0.0",
  "scripts": {
    "type-check": "tsc --noEmit"
  },
  "devDependencies": {
    "typescript": "^5.0.0"
  }
}`
	if err := os.WriteFile(filepath.Join(frontendDir, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatalf("failed to create package.json: %v", err)
	}
	t.Log("Step 1: Created package.json with TypeScript in frontend/")

	// Run pnpm install IN THE FRONTEND DIRECTORY
	t.Log("Step 2: Running pnpm install in frontend/ (this may take a moment)...")
	installCmd := exec.Command("pnpm", "install")
	installCmd.Dir = frontendDir
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
		t.Fatalf("failed to run pnpm install: %v", err)
	}
	t.Log("Step 2: pnpm install completed")

	// Verify tsc was installed
	tscPath := filepath.Join(frontendDir, "node_modules", ".bin", "tsc")
	env.AssertFileExists(tscPath)
	t.Logf("Step 2: Verified tsc exists at %s", tscPath)

	// Build ribbin binary
	ribbinPath := filepath.Join(env.TmpDir, "ribbin")
	buildCmd := exec.Command("go", "build", "-o", ribbinPath, "./cmd/ribbin")
	buildCmd.Dir = env.ModuleRoot
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build ribbin: %v\n%s", err, output)
	}
	env.RibbinPath = ribbinPath
	t.Log("Step 3: Built ribbin binary")

	// Create ribbin.jsonc at PROJECT ROOT with wrapper in FRONTEND SCOPE
	configContent := fmt.Sprintf(`{
  "scopes": {
    "frontend": {
      "path": "frontend",
      "wrappers": {
        "tsc": {
          "action": "block",
          "message": "Use 'pnpm run type-check' instead",
          "paths": ["%s"]
        }
      }
    }
  }
}`, tscPath)
	configPath := filepath.Join(projectDir, "ribbin.jsonc")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to create ribbin.jsonc: %v", err)
	}
	t.Log("Step 4: Created ribbin.jsonc")

	// Commit config file
	env.GitAdd(projectDir, "ribbin.jsonc")
	env.GitCommit(projectDir, "Add ribbin config")

	// Wrap tsc
	t.Log("Step 5: Running ribbin wrap...")
	wrapCmd := exec.Command(ribbinPath, "wrap")
	wrapCmd.Dir = projectDir
	wrapOutput, err := wrapCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to wrap: %v\n%s", err, wrapOutput)
	}
	t.Logf("Step 5: Wrap output:\n%s", wrapOutput)

	// Verify wrap was successful
	env.AssertOutputContains(string(wrapOutput), "1 wrapped")

	// Verify sidecar exists
	sidecarPath := tscPath + ".ribbin-original"
	env.AssertFileExists(sidecarPath)
	t.Logf("Step 5: Verified sidecar exists at %s", sidecarPath)

	// Verify symlink
	env.AssertSymlink(tscPath, ribbinPath)
	t.Log("Step 5: Verified tsc is now a symlink")

	// Activate ribbin globally
	t.Log("Step 5b: Activating ribbin globally...")
	activateCmd := exec.Command(ribbinPath, "activate", "--global")
	activateCmd.Dir = projectDir
	if activateOutput, err := activateCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to activate ribbin: %v\n%s", err, activateOutput)
	}
	t.Log("Step 5b: Activated ribbin globally")

	// Test that pnpm exec tsc FROM FRONTEND DIR is blocked (in scope)
	t.Log("Step 6: Testing pnpm exec tsc from frontend/ (should be blocked - in scope)...")
	execCmd := exec.Command("pnpm", "exec", "tsc", "--version")
	execCmd.Dir = frontendDir
	execOutput, execErr := execCmd.CombinedOutput()

	if execErr == nil {
		t.Fatalf("expected tsc to be blocked from frontend/, but it succeeded: %s", execOutput)
	}

	outputStr := string(execOutput)
	if !strings.Contains(outputStr, "Use 'pnpm run type-check' instead") {
		t.Fatalf("expected block message in output, got: %s", outputStr)
	}
	t.Log("Step 6: ✓ tsc was blocked from frontend/ (scope matched)")

	// Test that pnpm run type-check from frontend/ is also blocked
	t.Log("Step 7: Testing pnpm run type-check from frontend/ (should also be blocked)...")
	runCmd := exec.Command("pnpm", "run", "type-check")
	runCmd.Dir = frontendDir
	runOutput, runErr := runCmd.CombinedOutput()

	if runErr == nil {
		t.Fatalf("expected type-check to be blocked, but it succeeded")
	}

	if !strings.Contains(string(runOutput), "Use 'pnpm run type-check' instead") {
		t.Fatalf("expected block message in output, got: %s", runOutput)
	}
	t.Log("Step 7: ✓ pnpm run type-check was blocked from frontend/")

	// Unwrap
	t.Log("Step 8: Running ribbin unwrap...")
	unwrapCmd := exec.Command(ribbinPath, "unwrap")
	unwrapCmd.Dir = projectDir
	unwrapOutput, err := unwrapCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to unwrap: %v\n%s", err, unwrapOutput)
	}
	t.Logf("Step 8: Unwrap output:\n%s", unwrapOutput)

	// Verify tsc is restored
	env.AssertNotSymlink(tscPath)
	t.Log("Step 8: ✓ tsc restored to original")

	// Verify tsc works normally now
	t.Log("Step 9: Testing pnpm exec tsc after unwrap (should work normally)...")
	finalCmd := exec.Command("pnpm", "exec", "tsc", "--version")
	finalCmd.Dir = frontendDir
	finalOutput, finalErr := finalCmd.CombinedOutput()

	if finalErr != nil {
		t.Errorf("expected tsc to work after unwrap: %v\n%s", finalErr, finalOutput)
	} else {
		env.AssertOutputContains(string(finalOutput), "Version")
		t.Log("Step 9: ✓ tsc works normally after unwrap")
	}

	t.Log("Real pnpm scope wrappers test completed!")
}

// TestSystemBinaryWrapping tests wrapping a system-installed binary.
func TestSystemBinaryWrapping(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	env.SetPathWithBinDir()

	localBinDir := env.CreateDir("local-bin") // simulates /usr/local/bin

	// Build ribbin
	env.BuildRibbin("")

	// Create a wrapper script that mimics a system binary
	localBinaryPath := filepath.Join(localBinDir, "mytool")
	binaryContent := `#!/bin/sh
echo "system mytool v1.0.0"
exit 0
`
	if err := os.WriteFile(localBinaryPath, []byte(binaryContent), 0755); err != nil {
		t.Fatalf("failed to write binary: %v", err)
	}

	// Verify it works before shimming
	t.Log("Verifying mytool works before shimming...")
	cmd := exec.Command(localBinaryPath)
	cmd.Env = env.EnvironWithPath(localBinDir + ":" + env.BinDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("mytool failed before shimming: %v\n%s", err, output)
	}
	t.Logf("mytool output: %s", output)

	// Init git repo and create config
	env.InitGitRepo(env.ProjectDir)
	env.Chdir(env.ProjectDir)

	configPath := env.CreateBlockConfig(env.ProjectDir, "mytool", "System mytool is blocked in this project", []string{localBinaryPath})
	env.GitAdd(env.ProjectDir, "ribbin.jsonc")
	env.GitCommit(env.ProjectDir, "Add config")

	// Wrap mytool
	t.Log("Running ribbin wrap...")
	wrapCmd := exec.Command(env.RibbinPath, "wrap", "--confirm-system-dir", configPath)
	wrapCmd.Dir = env.ProjectDir
	wrapCmd.Env = env.EnvironWithPath(localBinDir + ":" + env.BinDir)
	output, err = wrapCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ribbin wrap failed: %v\n%s", err, output)
	}
	t.Logf("ribbin wrap output: %s", output)

	// Activate globally
	t.Log("Running ribbin activate --global...")
	activateCmd := exec.Command(env.RibbinPath, "activate", "--global")
	activateCmd.Dir = env.ProjectDir
	activateCmd.Env = env.EnvironWithPath(localBinDir + ":" + env.BinDir)
	output, err = activateCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ribbin activate --global failed: %v\n%s", err, output)
	}
	t.Logf("ribbin activate output: %s", output)

	// Verify sidecar exists
	sidecarPath := localBinaryPath + ".ribbin-original"
	env.AssertFileExists(sidecarPath)
	t.Logf("Sidecar exists: %s", sidecarPath)

	// Test: mytool should be blocked
	t.Log("Test: mytool should be blocked")
	cmd = exec.Command(localBinaryPath)
	cmd.Env = env.EnvironWithPath(localBinDir + ":" + env.BinDir)
	output, err = cmd.CombinedOutput()
	if err == nil {
		t.Errorf("mytool should be blocked, but succeeded: %s", output)
	} else {
		t.Logf("mytool blocked: %s", output)
	}

	// Test: bypass should work
	t.Log("Test: bypass should work")
	cmd = exec.Command(localBinaryPath)
	cmd.Env = append(env.EnvironWithPath(localBinDir+":"+env.BinDir), "RIBBIN_BYPASS=1")
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Errorf("mytool with bypass should work: %v\n%s", err, output)
	} else {
		env.AssertOutputContains(string(output), "system mytool")
		t.Logf("bypass works: %s", output)
	}

	// Unwrap
	t.Log("Running ribbin unwrap...")
	unwrapCmd := exec.Command(env.RibbinPath, "unwrap", configPath)
	unwrapCmd.Dir = env.ProjectDir
	unwrapCmd.Env = env.EnvironWithPath(localBinDir + ":" + env.BinDir)
	output, err = unwrapCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ribbin unwrap failed: %v\n%s", err, output)
	}
	t.Logf("ribbin unwrap output: %s", output)

	// Verify restoration
	env.AssertFileExists(localBinaryPath)
	env.AssertFileNotExists(sidecarPath)

	// Verify it works after unshimming
	cmd = exec.Command(localBinaryPath)
	cmd.Env = env.EnvironWithPath(localBinDir + ":" + env.BinDir)
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Errorf("mytool should work after unshimming: %v\n%s", err, output)
	} else {
		t.Logf("mytool restored: %s", output)
	}

	t.Log("system binary test completed!")
}
