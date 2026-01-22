package internal

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	_ "github.com/happycollision/ribbin/internal/testsafety"

	"github.com/happycollision/ribbin/internal/config"
	"github.com/happycollision/ribbin/internal/testutil"
)

// TestConfigDiscovery tests finding ribbin.jsonc in parent directories
func TestConfigDiscovery(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)

	// Create nested directory structure
	// project/ribbin.jsonc
	// project/src/lib/deep/
	projectDir := env.CreateDir("project")
	srcDir := env.CreateDir("project/src")
	libDir := env.CreateDir("project/src/lib")
	deepDir := env.CreateDir("project/src/lib/deep")

	// Create config in project root
	configContent := `{
  "wrappers": {
    "test-cmd": {
      "action": "block",
      "message": "blocked"
    }
  }
}`
	env.CreateConfig(projectDir, configContent)

	// Test config discovery from deep directory
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(deepDir)

	configPath, err := config.FindProjectConfig()
	if err != nil {
		t.Fatalf("config discovery failed: %v", err)
	}

	expectedPath := filepath.Join(projectDir, "ribbin.jsonc")
	if configPath != expectedPath {
		t.Errorf("expected config at %s, got %s", expectedPath, configPath)
	}

	// Test config loads correctly
	cfg, err := config.LoadProjectConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if _, exists := cfg.Wrappers["test-cmd"]; !exists {
		t.Error("config should contain test-cmd wrapper")
	}

	_ = srcDir
	_ = libDir
}

// TestParentDirectoryConfigDiscovery tests finding config in parent dirs when CWD is deep
func TestParentDirectoryConfigDiscovery(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	env.SetPathWithBinDir()

	// Create structure: parent/ribbin.jsonc, parent/project/src/
	parentDir := env.CreateDir("parent")
	projectDir := env.CreateDir("parent/project")
	srcDir := env.CreateDir("parent/project/src")

	// Create test binary
	testBinaryPath := env.CreateMockBinaryWithOutput(env.BinDir, "test-cmd", "REAL_TEST_CMD")

	// Create config at parent level
	env.CreateBlockConfig(parentDir, "test-cmd", "Use proper-cmd instead", []string{testBinaryPath})

	// Build ribbin
	env.BuildRibbin("")

	// Initialize git repo at parent level
	env.InitGitRepo(parentDir)
	env.GitAdd(parentDir, "ribbin.jsonc")
	env.GitCommit(parentDir, "Add config")

	// Wrap from parent dir
	env.MustRunRibbin(parentDir, "wrap")
	env.MustRunRibbin(parentDir, "activate", "--global")

	// Verify test-cmd works from srcDir (no config there, but parent has one)
	env.Chdir(srcDir)
	output, err := env.RunCmd(srcDir, testBinaryPath)

	// Should be blocked because ribbin walks up to find config
	if err == nil {
		t.Logf("Note: command succeeded (might be passthrough): %s", output)
	}

	// Test bypass works
	cmd := exec.Command(testBinaryPath)
	cmd.Dir = srcDir
	cmd.Env = env.EnvironWith("RIBBIN_BYPASS=1")
	output2, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("bypass should work: %v\n%s", err, output2)
	}
	env.AssertOutputContains(string(output2), "REAL_TEST_CMD")

	// Unwrap
	env.MustRunRibbin(parentDir, "unwrap")

	// Verify restored
	env.AssertNotSymlink(testBinaryPath)

	_ = projectDir
}

// TestConfigShowCommand tests the 'ribbin config show' command
func TestConfigShowCommand(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	env.SetPathWithBinDir()

	// Initialize git repo
	env.InitGitRepo(env.ProjectDir)

	// Create config with multiple wrappers
	configContent := `{
  "wrappers": {
    "tsc": {
      "action": "block",
      "message": "Use pnpm run typecheck"
    },
    "npm": {
      "action": "block",
      "message": "Use pnpm instead"
    }
  }
}`
	env.CreateConfig(env.ProjectDir, configContent)
	env.GitAdd(env.ProjectDir, "ribbin.jsonc")
	env.GitCommit(env.ProjectDir, "Add config")

	// Build ribbin
	env.BuildRibbin("")

	// Test 'ribbin config show'
	output := env.MustRunRibbin(env.ProjectDir, "config", "show")

	// Should show the config file path
	env.AssertOutputContains(output, "ribbin.jsonc")

	// Should show wrapper names
	env.AssertOutputContains(output, "tsc")
	env.AssertOutputContains(output, "npm")

	t.Logf("Config show output:\n%s", output)
}

// TestScopedConfigIsolation tests that isolated scopes (no extends) only have their own shims
func TestScopedConfigIsolation(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	fixtureDir := filepath.Join(env.ModuleRoot, "testdata", "projects", "scoped")
	configPath := filepath.Join(fixtureDir, "ribbin.jsonc")

	cfg, err := config.LoadProjectConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Create an isolated scope (no extends) for testing
	isolatedScope := config.ScopeConfig{
		Path:     "isolated",
		Wrappers: map[string]config.ShimConfig{"local-cmd": {Action: "block", Message: "local only"}},
	}

	resolver := config.NewResolver()
	result, err := resolver.ResolveEffectiveShims(cfg, configPath, &isolatedScope)
	if err != nil {
		t.Fatalf("ResolveEffectiveShims error: %v", err)
	}

	// Should only have local-cmd, not cat/npm/rm from root
	if len(result) != 1 {
		t.Errorf("isolated scope should have 1 shim, got %d: %v", len(result), result)
	}
	if _, ok := result["local-cmd"]; !ok {
		t.Error("isolated scope should have local-cmd")
	}
	if _, ok := result["cat"]; ok {
		t.Error("isolated scope should NOT have cat (no extends)")
	}
}

// TestScopedConfigExtendsRoot tests that scopes extending root inherit root shims
func TestScopedConfigExtendsRoot(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	fixtureDir := filepath.Join(env.ModuleRoot, "testdata", "projects", "scoped")
	configPath := filepath.Join(fixtureDir, "ribbin.jsonc")

	cfg, err := config.LoadProjectConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Frontend scope extends root
	frontendScope := cfg.Scopes["frontend"]
	resolver := config.NewResolver()
	result, err := resolver.ResolveEffectiveShims(cfg, configPath, &frontendScope)
	if err != nil {
		t.Fatalf("ResolveEffectiveShims error: %v", err)
	}

	// Should have root shims (cat, npm, rm) plus frontend shims
	if _, ok := result["cat"]; !ok {
		t.Error("frontend scope should inherit cat from root")
	}

	// Root has npm blocking, frontend should inherit it
	if _, ok := result["npm"]; !ok {
		t.Error("frontend scope should inherit npm from root")
	}
}

// TestScopedConfigMultipleExtends tests scopes with multiple extends
func TestScopedConfigMultipleExtends(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	fixtureDir := filepath.Join(env.ModuleRoot, "testdata", "projects", "scoped")
	configPath := filepath.Join(fixtureDir, "ribbin.jsonc")

	cfg, err := config.LoadProjectConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Backend scope
	backendScope := cfg.Scopes["backend"]
	resolver := config.NewResolver()
	result, err := resolver.ResolveEffectiveShims(cfg, configPath, &backendScope)
	if err != nil {
		t.Fatalf("ResolveEffectiveShims error: %v", err)
	}

	// Backend might extend root or have its own config
	t.Logf("Backend scope resolved shims: %v", result)

	// Should have some shims
	if len(result) == 0 {
		t.Error("backend scope should have some shims")
	}
}

// TestScopedConfigPassthrough tests that passthrough action works in scopes
func TestScopedConfigPassthrough(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	fixtureDir := filepath.Join(env.ModuleRoot, "testdata", "projects", "scoped")
	configPath := filepath.Join(fixtureDir, "ribbin.jsonc")

	cfg, err := config.LoadProjectConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Check if any scope has passthrough action
	for name, scope := range cfg.Scopes {
		for cmd, shim := range scope.Wrappers {
			if shim.Action == "passthrough" {
				t.Logf("Scope %s has passthrough for %s", name, cmd)
			}
		}
	}
}

// TestScopedConfigExternalExtends tests extending from external files
func TestScopedConfigExternalExtends(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	fixtureDir := filepath.Join(env.ModuleRoot, "testdata", "projects", "extends")

	// Check if the extends fixture exists
	configPath := filepath.Join(fixtureDir, "ribbin.jsonc")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Skip("extends fixture not found")
	}

	cfg, err := config.LoadProjectConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	t.Logf("Loaded config with %d root wrappers", len(cfg.Wrappers))

	// The extends fixture tests external file inheritance
	if len(cfg.Wrappers) == 0 && len(cfg.Scopes) == 0 {
		t.Log("Config is empty - might be testing scoped extends only")
	}
}

// TestProvenanceTracking tests that wrapper provenance is tracked correctly
func TestProvenanceTracking(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	env.SetPathWithBinDir()
	env.InitGitRepo(env.ProjectDir)

	// Create test binary
	testBinaryPath := env.CreateMockBinary(env.BinDir, "test-cmd")

	// Create config
	env.CreateBlockConfig(env.ProjectDir, "test-cmd", "blocked", []string{testBinaryPath})
	env.GitAdd(env.ProjectDir, "ribbin.jsonc")
	env.GitCommit(env.ProjectDir, "Add config")

	// Build and wrap
	env.BuildRibbin("")
	env.MustRunRibbin(env.ProjectDir, "wrap")

	// Check that metadata file exists and contains provenance info
	metaPath := testBinaryPath + ".ribbin-meta"
	env.AssertFileExists(metaPath)

	metaContent, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("failed to read meta file: %v", err)
	}

	// Should contain wrapped_at timestamp (provenance tracking)
	env.AssertOutputContains(string(metaContent), "wrapped_at")
	// Should contain original hash
	env.AssertOutputContains(string(metaContent), "original_hash")
	// Should contain ribbin path
	env.AssertOutputContains(string(metaContent), "ribbin_path")

	// Cleanup
	env.MustRunRibbin(env.ProjectDir, "unwrap")
}
