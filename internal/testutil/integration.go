// Package testutil provides test helpers for ribbin tests.
package testutil

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/happycollision/ribbin/internal/config"
)

// IntegrationEnv represents a complete isolated test environment for integration tests.
type IntegrationEnv struct {
	T          *testing.T
	TmpDir     string
	HomeDir    string
	ProjectDir string
	BinDir     string
	RibbinPath string
	ModuleRoot string

	// Saved environment for restoration
	origHome string
	origPath string
	origDir  string
}

// SetupIntegrationEnv creates a complete isolated test environment.
// It creates temp directories, saves environment variables, and builds ribbin.
// The environment is automatically cleaned up when the test completes.
func SetupIntegrationEnv(t *testing.T) *IntegrationEnv {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "ribbin-integration-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	env := &IntegrationEnv{
		T:          t,
		TmpDir:     tmpDir,
		HomeDir:    filepath.Join(tmpDir, "home"),
		ProjectDir: filepath.Join(tmpDir, "project"),
		BinDir:     filepath.Join(tmpDir, "bin"),
	}

	// Create directories
	for _, dir := range []string{env.HomeDir, env.ProjectDir, env.BinDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			os.RemoveAll(tmpDir)
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	// Find module root before changing directories
	env.ModuleRoot = findModuleRoot(t)

	// Save original environment
	env.origHome = os.Getenv("HOME")
	env.origPath = os.Getenv("PATH")
	env.origDir, _ = os.Getwd()

	// Set up cleanup
	t.Cleanup(func() {
		os.Setenv("HOME", env.origHome)
		os.Setenv("PATH", env.origPath)
		os.Chdir(env.origDir)
		os.RemoveAll(tmpDir)
	})

	// Set environment
	os.Setenv("HOME", env.HomeDir)

	return env
}

// SetPathWithBinDir sets PATH to include the test bin directory.
func (env *IntegrationEnv) SetPathWithBinDir() {
	os.Setenv("PATH", env.BinDir+":"+env.origPath)
}

// GetOrigPath returns the original PATH value.
func (env *IntegrationEnv) GetOrigPath() string {
	return env.origPath
}

// BuildRibbin builds the ribbin binary and places it in the specified directory.
// If dir is empty, uses env.BinDir.
func (env *IntegrationEnv) BuildRibbin(dir string) string {
	env.T.Helper()

	if dir == "" {
		dir = env.BinDir
	}

	ribbinPath := filepath.Join(dir, "ribbin")
	buildCmd := exec.Command("go", "build", "-o", ribbinPath, "./cmd/ribbin")
	buildCmd.Dir = env.ModuleRoot
	if output, err := buildCmd.CombinedOutput(); err != nil {
		env.T.Fatalf("failed to build ribbin: %v\n%s", err, output)
	}

	env.RibbinPath = ribbinPath
	return ribbinPath
}

// Chdir changes to the specified directory (relative to TmpDir if not absolute).
func (env *IntegrationEnv) Chdir(dir string) {
	env.T.Helper()
	if err := os.Chdir(dir); err != nil {
		env.T.Fatalf("failed to chdir to %s: %v", dir, err)
	}
}

// ChdirProject changes to the project directory.
func (env *IntegrationEnv) ChdirProject() {
	env.Chdir(env.ProjectDir)
}

// CreateDir creates a directory under TmpDir.
func (env *IntegrationEnv) CreateDir(relPath string) string {
	env.T.Helper()
	dir := filepath.Join(env.TmpDir, relPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		env.T.Fatalf("failed to create dir %s: %v", dir, err)
	}
	return dir
}

// CreateMockBinary creates a simple shell script binary that echoes its name and args.
func (env *IntegrationEnv) CreateMockBinary(dir, name string) string {
	env.T.Helper()
	return env.CreateMockBinaryWithOutput(dir, name, name+" executed")
}

// CreateMockBinaryWithOutput creates a shell script binary with custom output.
func (env *IntegrationEnv) CreateMockBinaryWithOutput(dir, name, output string) string {
	env.T.Helper()
	path := filepath.Join(dir, name)
	content := "#!/bin/sh\necho \"" + output + "\"\nexit 0\n"
	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		env.T.Fatalf("failed to create mock binary %s: %v", path, err)
	}
	return path
}

// CreateConfig creates a ribbin.jsonc file with the given content.
func (env *IntegrationEnv) CreateConfig(dir, content string) string {
	env.T.Helper()
	path := filepath.Join(dir, "ribbin.jsonc")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		env.T.Fatalf("failed to create config: %v", err)
	}
	return path
}

// CreateBlockConfig creates a simple block config for the given command and paths.
func (env *IntegrationEnv) CreateBlockConfig(dir, cmdName, message string, paths []string) string {
	env.T.Helper()

	pathsJSON := "["
	for i, p := range paths {
		if i > 0 {
			pathsJSON += ", "
		}
		pathsJSON += "\"" + p + "\""
	}
	pathsJSON += "]"

	content := `{
  "wrappers": {
    "` + cmdName + `": {
      "action": "block",
      "message": "` + message + `",
      "paths": ` + pathsJSON + `
    }
  }
}`
	return env.CreateConfig(dir, content)
}

// InitGitRepo initializes a git repo in the specified directory.
func (env *IntegrationEnv) InitGitRepo(dir string) {
	env.T.Helper()

	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		env.T.Fatalf("failed to init git repo in %s: %v", dir, err)
	}

	// Configure git user for commits
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = dir
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = dir
	cmd.Run()
}

// GitAdd stages a file.
func (env *IntegrationEnv) GitAdd(dir, file string) {
	env.T.Helper()
	cmd := exec.Command("git", "add", file)
	cmd.Dir = dir
	cmd.Run()
}

// GitCommit creates a commit.
func (env *IntegrationEnv) GitCommit(dir, message string) {
	env.T.Helper()
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = dir
	cmd.Run()
}

// NewRegistry creates an empty registry.
func (env *IntegrationEnv) NewRegistry() *config.Registry {
	return &config.Registry{
		Wrappers:          make(map[string]config.WrapperEntry),
		ShellActivations:  make(map[int]config.ShellActivationEntry),
		ConfigActivations: make(map[string]config.ConfigActivationEntry),
		GlobalActive:      false,
	}
}

// SaveRegistry saves a registry to the home config directory.
func (env *IntegrationEnv) SaveRegistry(registry *config.Registry) string {
	env.T.Helper()

	registryDir := filepath.Join(env.HomeDir, ".config", "ribbin")
	if err := os.MkdirAll(registryDir, 0755); err != nil {
		env.T.Fatalf("failed to create registry dir: %v", err)
	}

	registryPath := filepath.Join(registryDir, "registry.json")
	data, _ := json.MarshalIndent(registry, "", "  ")
	if err := os.WriteFile(registryPath, data, 0644); err != nil {
		env.T.Fatalf("failed to save registry: %v", err)
	}

	return registryPath
}

// LoadRegistry loads the registry from the home config directory.
func (env *IntegrationEnv) LoadRegistry() *config.Registry {
	env.T.Helper()

	registryPath := filepath.Join(env.HomeDir, ".config", "ribbin", "registry.json")
	data, err := os.ReadFile(registryPath)
	if err != nil {
		env.T.Fatalf("failed to read registry: %v", err)
	}

	var registry config.Registry
	if err := json.Unmarshal(data, &registry); err != nil {
		env.T.Fatalf("failed to parse registry: %v", err)
	}

	return &registry
}

// RunRibbin runs a ribbin command and returns the output.
func (env *IntegrationEnv) RunRibbin(dir string, args ...string) (string, error) {
	env.T.Helper()

	cmd := exec.Command(env.RibbinPath, args...)
	cmd.Dir = dir
	cmd.Env = env.Environ()
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// MustRunRibbin runs a ribbin command and fails if it errors.
func (env *IntegrationEnv) MustRunRibbin(dir string, args ...string) string {
	env.T.Helper()
	output, err := env.RunRibbin(dir, args...)
	if err != nil {
		env.T.Fatalf("ribbin %v failed: %v\n%s", args, err, output)
	}
	return output
}

// RunCmd runs an arbitrary command with the test environment.
func (env *IntegrationEnv) RunCmd(dir, name string, args ...string) (string, error) {
	env.T.Helper()

	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = env.Environ()
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// MustRunCmd runs an arbitrary command and fails if it errors.
func (env *IntegrationEnv) MustRunCmd(dir, name string, args ...string) string {
	env.T.Helper()
	output, err := env.RunCmd(dir, name, args...)
	if err != nil {
		env.T.Fatalf("%s %v failed: %v\n%s", name, args, err, output)
	}
	return output
}

// Environ returns the test environment variables.
func (env *IntegrationEnv) Environ() []string {
	return append(os.Environ(),
		"HOME="+env.HomeDir,
		"PATH="+env.BinDir+":"+env.origPath,
	)
}

// EnvironWith returns the test environment with additional variables.
func (env *IntegrationEnv) EnvironWith(extra ...string) []string {
	return append(env.Environ(), extra...)
}

// EnvironWithPath returns the test environment with a custom PATH prefix.
func (env *IntegrationEnv) EnvironWithPath(pathPrefix string) []string {
	return append(os.Environ(),
		"HOME="+env.HomeDir,
		"PATH="+pathPrefix+":"+env.origPath,
	)
}

// AssertSymlink asserts that path is a symlink pointing to target.
func (env *IntegrationEnv) AssertSymlink(path, target string) {
	env.T.Helper()
	linkTarget, err := os.Readlink(path)
	if err != nil {
		env.T.Fatalf("%s should be a symlink: %v", path, err)
	}
	if linkTarget != target {
		env.T.Errorf("%s should point to %s, got %s", path, target, linkTarget)
	}
}

// AssertNotSymlink asserts that path exists and is not a symlink.
func (env *IntegrationEnv) AssertNotSymlink(path string) {
	env.T.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		env.T.Fatalf("%s should exist: %v", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		env.T.Errorf("%s should not be a symlink", path)
	}
}

// AssertFileExists asserts that a file exists.
func (env *IntegrationEnv) AssertFileExists(path string) {
	env.T.Helper()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		env.T.Errorf("expected %s to exist", path)
	}
}

// AssertFileNotExists asserts that a file does not exist.
func (env *IntegrationEnv) AssertFileNotExists(path string) {
	env.T.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		env.T.Errorf("expected %s to not exist", path)
	}
}

// AssertOutputContains asserts that output contains the substring.
func (env *IntegrationEnv) AssertOutputContains(output, substr string) {
	env.T.Helper()
	if !strings.Contains(output, substr) {
		env.T.Errorf("expected output to contain %q, got: %s", substr, output)
	}
}

// AssertOutputNotContains asserts that output does not contain the substring.
func (env *IntegrationEnv) AssertOutputNotContains(output, substr string) {
	env.T.Helper()
	if strings.Contains(output, substr) {
		env.T.Errorf("expected output to not contain %q, got: %s", substr, output)
	}
}

// findModuleRoot finds the Go module root directory.
func findModuleRoot(t *testing.T) string {
	t.Helper()

	// Start from the current file's location
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find module root (go.mod)")
		}
		dir = parent
	}
}

// Contains checks if s contains substr (utility for tests).
func Contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
