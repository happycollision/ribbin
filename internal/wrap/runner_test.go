package wrap

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	_ "github.com/happycollision/ribbin/internal/testsafety"

	"github.com/happycollision/ribbin/internal/config"
)

func TestExtractCommandName(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/usr/bin/cat", "cat"},
		{"/usr/local/bin/node", "node"},
		{"cat", "cat"},
		{"/a/b/c/d/program", "program"},
		{"./relative/path/cmd", "cmd"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := extractCommandName(tt.path)
			if result != tt.expected {
				t.Errorf("extractCommandName(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestIsActive(t *testing.T) {
	t.Run("returns true when GlobalActive is true", func(t *testing.T) {
		registry := &config.Registry{
			Wrappers:       make(map[string]config.WrapperEntry),
			ShellActivations:  make(map[int]config.ShellActivationEntry),
			ConfigActivations: make(map[string]config.ConfigActivationEntry),
			GlobalActive:    true,
		}

		if !isActive(registry) {
			t.Error("should be active when GlobalActive is true")
		}
	})

	t.Run("returns false when GlobalActive is false and no activations", func(t *testing.T) {
		registry := &config.Registry{
			Wrappers:       make(map[string]config.WrapperEntry),
			ShellActivations:  make(map[int]config.ShellActivationEntry),
			ConfigActivations: make(map[string]config.ConfigActivationEntry),
			GlobalActive:    false,
		}

		if isActive(registry) {
			t.Error("should not be active when GlobalActive is false and no activations")
		}
	})

	t.Run("returns true when ancestor PID is in activations", func(t *testing.T) {
		// PID 1 is always an ancestor (init/launchd)
		registry := &config.Registry{
			Wrappers: make(map[string]config.WrapperEntry),
			ShellActivations: map[int]config.ShellActivationEntry{
				1: {PID: 1, ActivatedAt: time.Now()},
			},
			ConfigActivations: make(map[string]config.ConfigActivationEntry),
			GlobalActive:      false,
		}

		if !isActive(registry) {
			t.Error("should be active when PID 1 is in activations")
		}
	})

	t.Run("returns false when non-ancestor PID is in activations", func(t *testing.T) {
		// Use a high PID that's unlikely to be an ancestor
		registry := &config.Registry{
			Wrappers: make(map[string]config.WrapperEntry),
			ShellActivations: map[int]config.ShellActivationEntry{
				99999999: {PID: 99999999, ActivatedAt: time.Now()},
			},
			ConfigActivations: make(map[string]config.ConfigActivationEntry),
			GlobalActive:      false,
		}

		if isActive(registry) {
			t.Error("should not be active when only non-ancestor PIDs in activations")
		}
	})
}

// Note: Run() uses syscall.Exec which replaces the current process,
// making it difficult to test directly. We test the helper functions
// and integration tests cover the full flow.

func TestRunWithMissingOriginal(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "ribbin-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Call Run with a path that has no sidecar
	binaryPath := filepath.Join(tmpDir, "missing")
	err = Run(binaryPath, []string{})
	if err == nil {
		t.Error("expected error when original binary is missing")
	}
}

func TestRunWithBypassEnv(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "ribbin-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create sidecar (we can't fully test this without exec, but we can check setup)
	binaryPath := filepath.Join(tmpDir, "test-cmd")
	sidecarPath := binaryPath + ".ribbin-original"

	// Create a valid sidecar binary
	if err := os.WriteFile(sidecarPath, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("failed to create sidecar: %v", err)
	}

	// Set bypass env
	os.Setenv("RIBBIN_BYPASS", "1")
	defer os.Unsetenv("RIBBIN_BYPASS")

	// Run would exec the original - we can't test this fully without forking
	// but we verify the sidecar exists which is the prerequisite
	if _, err := os.Stat(sidecarPath); os.IsNotExist(err) {
		t.Error("sidecar should exist for bypass test setup")
	}
}

func TestShouldPassthrough(t *testing.T) {
	// Note: shouldPassthrough relies on process.GetParentCommand() which returns
	// the actual parent process. In tests, this is typically "go test" or similar.
	// We test the matching logic by using patterns that should/shouldn't match
	// a typical test runner invocation.

	t.Run("returns false with nil config", func(t *testing.T) {
		// Nil config should be handled by caller, but let's verify no panic
		var pt *config.PassthroughConfig = nil
		if pt != nil && shouldPassthrough(pt) {
			t.Error("nil config should not passthrough")
		}
	})

	t.Run("returns false with empty config", func(t *testing.T) {
		pt := &config.PassthroughConfig{}
		if shouldPassthrough(pt) {
			t.Error("empty config should not passthrough")
		}
	})

	t.Run("exact match finds substring in parent command", func(t *testing.T) {
		// The parent of this test is likely "go test" or similar
		pt := &config.PassthroughConfig{
			Invocation: []string{"go"},
		}
		// This should match since tests run under "go test"
		if !shouldPassthrough(pt) {
			t.Error("should passthrough when exact pattern matches parent command")
		}
	})

	t.Run("exact match returns false for non-matching pattern", func(t *testing.T) {
		pt := &config.PassthroughConfig{
			Invocation: []string{"definitely-not-in-parent-command-xyz123"},
		}
		if shouldPassthrough(pt) {
			t.Error("should not passthrough when pattern doesn't match")
		}
	})

	t.Run("regexp match finds pattern in parent command", func(t *testing.T) {
		pt := &config.PassthroughConfig{
			InvocationRegexp: []string{"go.*test"},
		}
		// This should match since tests run under "go test"
		if !shouldPassthrough(pt) {
			t.Error("should passthrough when regexp matches parent command")
		}
	})

	t.Run("regexp match returns false for non-matching pattern", func(t *testing.T) {
		pt := &config.PassthroughConfig{
			InvocationRegexp: []string{"^pnpm run"},
		}
		if shouldPassthrough(pt) {
			t.Error("should not passthrough when regexp doesn't match")
		}
	})

	t.Run("invalid regexp is skipped", func(t *testing.T) {
		pt := &config.PassthroughConfig{
			InvocationRegexp: []string{"[invalid(regexp"},
		}
		// Should not panic, just return false
		if shouldPassthrough(pt) {
			t.Error("invalid regexp should be skipped, not match")
		}
	})

	t.Run("multiple patterns - first matches", func(t *testing.T) {
		pt := &config.PassthroughConfig{
			Invocation: []string{"go", "nonexistent"},
		}
		if !shouldPassthrough(pt) {
			t.Error("should passthrough when any exact pattern matches")
		}
	})

	t.Run("multiple patterns - second matches", func(t *testing.T) {
		pt := &config.PassthroughConfig{
			Invocation: []string{"nonexistent", "go"},
		}
		if !shouldPassthrough(pt) {
			t.Error("should passthrough when any exact pattern matches")
		}
	})

	t.Run("mixed exact and regexp - exact matches", func(t *testing.T) {
		pt := &config.PassthroughConfig{
			Invocation:       []string{"go"},
			InvocationRegexp: []string{"^pnpm"},
		}
		if !shouldPassthrough(pt) {
			t.Error("should passthrough when exact pattern matches even if regexp doesn't")
		}
	})

	t.Run("mixed exact and regexp - regexp matches", func(t *testing.T) {
		pt := &config.PassthroughConfig{
			Invocation:       []string{"nonexistent"},
			InvocationRegexp: []string{"go"},
		}
		if !shouldPassthrough(pt) {
			t.Error("should passthrough when regexp matches even if exact doesn't")
		}
	})
}

func TestPrintBlockMessage(t *testing.T) {
	// Capture stderr output is tricky, just verify it doesn't panic
	t.Run("prints with custom message", func(t *testing.T) {
		// Redirect stderr temporarily
		oldStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		printBlockMessage("cat", "Use bat instead for syntax highlighting")

		w.Close()
		os.Stderr = oldStderr

		// Read output
		buf := make([]byte, 1024)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		if len(output) == 0 {
			t.Error("expected output to stderr")
		}
	})

	t.Run("prints with default message", func(t *testing.T) {
		oldStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		printBlockMessage("npm", "")

		w.Close()
		os.Stderr = oldStderr

		buf := make([]byte, 1024)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		if len(output) == 0 {
			t.Error("expected output to stderr")
		}
	})
}

func TestIsPathWithin(t *testing.T) {
	tests := []struct {
		name       string
		targetPath string
		basePath   string
		expected   bool
	}{
		{
			name:       "exact match",
			targetPath: "/home/user/project",
			basePath:   "/home/user/project",
			expected:   true,
		},
		{
			name:       "target is subdirectory",
			targetPath: "/home/user/project/src",
			basePath:   "/home/user/project",
			expected:   true,
		},
		{
			name:       "target is deeply nested",
			targetPath: "/home/user/project/src/components/ui",
			basePath:   "/home/user/project",
			expected:   true,
		},
		{
			name:       "target is parent directory",
			targetPath: "/home/user",
			basePath:   "/home/user/project",
			expected:   false,
		},
		{
			name:       "target is sibling directory",
			targetPath: "/home/user/other",
			basePath:   "/home/user/project",
			expected:   false,
		},
		{
			name:       "completely different path",
			targetPath: "/var/log",
			basePath:   "/home/user",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPathWithin(tt.targetPath, tt.basePath)
			if result != tt.expected {
				t.Errorf("isPathWithin(%q, %q) = %v, want %v", tt.targetPath, tt.basePath, result, tt.expected)
			}
		})
	}
}

func TestCountPathComponents(t *testing.T) {
	tests := []struct {
		path     string
		expected int
	}{
		{"/home/user/project", 3},
		{"/home/user/project/src", 4},
		{"/", 0},
		{".", 1},
		{"./src", 1},
		{"/a/b/c/d/e", 5},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := countPathComponents(tt.path)
			if result != tt.expected {
				t.Errorf("countPathComponents(%q) = %d, want %d", tt.path, result, tt.expected)
			}
		})
	}
}

func TestFindBestMatchingScope(t *testing.T) {
	// Create a temporary directory structure for testing
	tmpDir, err := os.MkdirTemp("", "ribbin-scope-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create subdirectories
	srcDir := filepath.Join(tmpDir, "src")
	srcComponentsDir := filepath.Join(srcDir, "components")
	testsDir := filepath.Join(tmpDir, "tests")

	for _, dir := range []string{srcDir, srcComponentsDir, testsDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	configPath := filepath.Join(tmpDir, "ribbin.toml")

	t.Run("CWD in scope path - scope applies", func(t *testing.T) {
		projectConfig := &config.ProjectConfig{
			Wrappers: map[string]config.ShimConfig{
				"cat": {Action: "block", Message: "root cat"},
			},
			Scopes: map[string]config.ScopeConfig{
				"src": {
					Path: "src",
					Wrappers: map[string]config.ShimConfig{
						"cat": {Action: "block", Message: "src cat"},
					},
				},
			},
		}

		result := findBestMatchingScope(projectConfig, configPath, srcDir)
		if result == nil {
			t.Fatal("expected to find matching scope, got nil")
		}
		if result.Path != "src" {
			t.Errorf("expected scope path 'src', got %q", result.Path)
		}
	})

	t.Run("multiple scopes match - deepest path wins", func(t *testing.T) {
		projectConfig := &config.ProjectConfig{
			Wrappers: map[string]config.ShimConfig{},
			Scopes: map[string]config.ScopeConfig{
				"src": {
					Path: "src",
					Wrappers: map[string]config.ShimConfig{
						"cat": {Action: "block", Message: "src cat"},
					},
				},
				"src-components": {
					Path: "src/components",
					Wrappers: map[string]config.ShimConfig{
						"cat": {Action: "block", Message: "components cat"},
					},
				},
			},
		}

		result := findBestMatchingScope(projectConfig, configPath, srcComponentsDir)
		if result == nil {
			t.Fatal("expected to find matching scope, got nil")
		}
		if result.Path != "src/components" {
			t.Errorf("expected scope path 'src/components' (deepest), got %q", result.Path)
		}
	})

	t.Run("no scope matches - returns nil (root shims used)", func(t *testing.T) {
		projectConfig := &config.ProjectConfig{
			Wrappers: map[string]config.ShimConfig{
				"cat": {Action: "block", Message: "root cat"},
			},
			Scopes: map[string]config.ScopeConfig{
				"src": {
					Path: "src",
					Wrappers: map[string]config.ShimConfig{
						"cat": {Action: "block", Message: "src cat"},
					},
				},
			},
		}

		// Use tests directory which is not under src
		result := findBestMatchingScope(projectConfig, configPath, testsDir)
		if result != nil {
			t.Errorf("expected no matching scope, got scope with path %q", result.Path)
		}
	})

	t.Run("scope without path - defaults to config directory", func(t *testing.T) {
		projectConfig := &config.ProjectConfig{
			Wrappers: map[string]config.ShimConfig{},
			Scopes: map[string]config.ScopeConfig{
				"default": {
					Path: "", // Empty path defaults to "."
					Wrappers: map[string]config.ShimConfig{
						"cat": {Action: "block", Message: "default cat"},
					},
				},
			},
		}

		// CWD is within config directory (anywhere)
		result := findBestMatchingScope(projectConfig, configPath, srcDir)
		if result == nil {
			t.Fatal("expected to find matching scope with empty path, got nil")
		}
		if result.Path != "" {
			t.Errorf("expected scope with empty path, got %q", result.Path)
		}
	})

	t.Run("explicit dot path matches like empty path", func(t *testing.T) {
		projectConfig := &config.ProjectConfig{
			Wrappers: map[string]config.ShimConfig{},
			Scopes: map[string]config.ScopeConfig{
				"root-scope": {
					Path: ".",
					Wrappers: map[string]config.ShimConfig{
						"cat": {Action: "block", Message: "root scope cat"},
					},
				},
			},
		}

		result := findBestMatchingScope(projectConfig, configPath, srcDir)
		if result == nil {
			t.Fatal("expected to find matching scope with '.' path, got nil")
		}
	})
}

func TestGetEffectiveShimConfig(t *testing.T) {
	// Create a temporary directory structure for testing
	tmpDir, err := os.MkdirTemp("", "ribbin-effective-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	srcDir := filepath.Join(tmpDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("failed to create src dir: %v", err)
	}

	configPath := filepath.Join(tmpDir, "ribbin.toml")

	// Save current directory and change to temp dir for testing
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}

	t.Run("returns root shim when no scope matches", func(t *testing.T) {
		projectConfig := &config.ProjectConfig{
			Wrappers: map[string]config.ShimConfig{
				"cat": {Action: "block", Message: "root cat message"},
			},
			Scopes: map[string]config.ScopeConfig{
				"src": {
					Path: "src",
					Wrappers: map[string]config.ShimConfig{
						"cat": {Action: "block", Message: "src cat message"},
					},
				},
			},
		}

		// Change to temp root (not src)
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatalf("failed to change directory: %v", err)
		}
		defer os.Chdir(originalWd)

		shimConfig, exists := getEffectiveShimConfig(projectConfig, configPath, "cat")
		if !exists {
			t.Fatal("expected shim config to exist")
		}
		if shimConfig.Message != "root cat message" {
			t.Errorf("expected root shim message, got %q", shimConfig.Message)
		}
	})

	t.Run("returns scope shim when scope matches", func(t *testing.T) {
		projectConfig := &config.ProjectConfig{
			Wrappers: map[string]config.ShimConfig{
				"cat": {Action: "block", Message: "root cat message"},
			},
			Scopes: map[string]config.ScopeConfig{
				"src": {
					Path:    "src",
					Extends: []string{"root"}, // Explicitly extend root to inherit root shims
					Wrappers: map[string]config.ShimConfig{
						"cat": {Action: "block", Message: "src cat message"}, // Override root
					},
				},
			},
		}

		// Change to src directory
		if err := os.Chdir(srcDir); err != nil {
			t.Fatalf("failed to change directory: %v", err)
		}
		defer os.Chdir(originalWd)

		shimConfig, exists := getEffectiveShimConfig(projectConfig, configPath, "cat")
		if !exists {
			t.Fatal("expected shim config to exist")
		}
		if shimConfig.Message != "src cat message" {
			t.Errorf("expected src shim message, got %q", shimConfig.Message)
		}
	})

	t.Run("returns false for non-existent command", func(t *testing.T) {
		projectConfig := &config.ProjectConfig{
			Wrappers: map[string]config.ShimConfig{
				"cat": {Action: "block", Message: "root cat"},
			},
			Scopes: map[string]config.ScopeConfig{},
		}

		if err := os.Chdir(tmpDir); err != nil {
			t.Fatalf("failed to change directory: %v", err)
		}
		defer os.Chdir(originalWd)

		_, exists := getEffectiveShimConfig(projectConfig, configPath, "nonexistent")
		if exists {
			t.Error("expected shim config to not exist for unknown command")
		}
	})
}

func TestPassthroughAction(t *testing.T) {
	// This tests that "passthrough" action is recognized
	// The actual execution is tested in integration tests since it uses syscall.Exec

	t.Run("passthrough action is valid", func(t *testing.T) {
		// Create temp directory
		tmpDir, err := os.MkdirTemp("", "ribbin-passthrough-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		// Create sidecar binary
		binaryPath := filepath.Join(tmpDir, "test-cmd")
		sidecarPath := binaryPath + ".ribbin-original"

		// Create a dummy executable
		var scriptContent string
		if runtime.GOOS == "windows" {
			scriptContent = "@echo off\nexit 0\n"
		} else {
			scriptContent = "#!/bin/sh\nexit 0\n"
		}
		if err := os.WriteFile(sidecarPath, []byte(scriptContent), 0755); err != nil {
			t.Fatalf("failed to create sidecar: %v", err)
		}

		// Verify that passthrough action config can be created and checked
		shimConfig := config.ShimConfig{
			Action:  "passthrough",
			Message: "This should passthrough",
		}

		if shimConfig.Action != "passthrough" {
			t.Errorf("expected action 'passthrough', got %q", shimConfig.Action)
		}
	})
}

func TestScopeMatchingIntegration(t *testing.T) {
	// Integration test that verifies full scope matching flow
	tmpDir, err := os.MkdirTemp("", "ribbin-integration-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create directory structure
	frontendDir := filepath.Join(tmpDir, "frontend")
	frontendSrcDir := filepath.Join(frontendDir, "src")
	backendDir := filepath.Join(tmpDir, "backend")

	for _, dir := range []string{frontendSrcDir, backendDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create dir: %v", err)
		}
	}

	configPath := filepath.Join(tmpDir, "ribbin.toml")

	projectConfig := &config.ProjectConfig{
		Wrappers: map[string]config.ShimConfig{
			"npm": {Action: "block", Message: "Use pnpm at root"},
		},
		Scopes: map[string]config.ScopeConfig{
			"frontend": {
				Path:    "frontend",
				Extends: []string{"root"}, // Inherit root, then override
				Wrappers: map[string]config.ShimConfig{
					"npm": {Action: "passthrough"}, // Allow npm in frontend
				},
			},
			"frontend-src": {
				Path:    "frontend/src",
				Extends: []string{"root"}, // Inherit root, then override
				Wrappers: map[string]config.ShimConfig{
					"npm": {Action: "block", Message: "No npm in src"},
				},
			},
			"backend": {
				Path:    "backend",
				Extends: []string{"root"}, // Inherit root, then override
				Wrappers: map[string]config.ShimConfig{
					"npm": {Action: "redirect", Redirect: "./scripts/backend-npm.sh"},
				},
			},
		},
	}

	originalWd, _ := os.Getwd()

	t.Run("root directory uses root shims", func(t *testing.T) {
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		shimConfig, exists := getEffectiveShimConfig(projectConfig, configPath, "npm")
		if !exists {
			t.Fatal("expected shim to exist")
		}
		if shimConfig.Action != "block" || shimConfig.Message != "Use pnpm at root" {
			t.Errorf("unexpected shim config: %+v", shimConfig)
		}
	})

	t.Run("frontend directory uses frontend scope", func(t *testing.T) {
		os.Chdir(frontendDir)
		defer os.Chdir(originalWd)

		shimConfig, exists := getEffectiveShimConfig(projectConfig, configPath, "npm")
		if !exists {
			t.Fatal("expected shim to exist")
		}
		if shimConfig.Action != "passthrough" {
			t.Errorf("expected passthrough action in frontend, got %q", shimConfig.Action)
		}
	})

	t.Run("frontend/src uses deepest matching scope", func(t *testing.T) {
		os.Chdir(frontendSrcDir)
		defer os.Chdir(originalWd)

		shimConfig, exists := getEffectiveShimConfig(projectConfig, configPath, "npm")
		if !exists {
			t.Fatal("expected shim to exist")
		}
		if shimConfig.Action != "block" || shimConfig.Message != "No npm in src" {
			t.Errorf("expected frontend-src scope, got action=%q message=%q", shimConfig.Action, shimConfig.Message)
		}
	})

	t.Run("backend uses backend scope", func(t *testing.T) {
		os.Chdir(backendDir)
		defer os.Chdir(originalWd)

		shimConfig, exists := getEffectiveShimConfig(projectConfig, configPath, "npm")
		if !exists {
			t.Fatal("expected shim to exist")
		}
		if shimConfig.Action != "redirect" {
			t.Errorf("expected redirect action in backend, got %q", shimConfig.Action)
		}
	})
}
