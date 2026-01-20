package config

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/happycollision/ribbin/internal/testsafety"
)

func TestFindProjectConfig(t *testing.T) {
	// Create a temp directory structure
	tmpDir, err := os.MkdirTemp("", "ribbin-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Resolve symlinks in tmpDir (on macOS, /var -> /private/var)
	tmpDir, err = filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatalf("failed to resolve symlinks: %v", err)
	}

	// Save original working directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(origDir)

	t.Run("finds config in current directory", func(t *testing.T) {
		// Create a subdirectory with config
		projectDir := filepath.Join(tmpDir, "project1")
		if err := os.MkdirAll(projectDir, 0755); err != nil {
			t.Fatalf("failed to create project dir: %v", err)
		}

		configPath := filepath.Join(projectDir, "ribbin.toml")
		if err := os.WriteFile(configPath, []byte("[shims]\n"), 0644); err != nil {
			t.Fatalf("failed to create config: %v", err)
		}

		if err := os.Chdir(projectDir); err != nil {
			t.Fatalf("failed to chdir: %v", err)
		}

		found, err := FindProjectConfig()
		if err != nil {
			t.Fatalf("FindProjectConfig error: %v", err)
		}
		if found != configPath {
			t.Errorf("expected %s, got %s", configPath, found)
		}
	})

	t.Run("finds config in parent directory", func(t *testing.T) {
		// Create nested directories
		parentDir := filepath.Join(tmpDir, "project2")
		childDir := filepath.Join(parentDir, "src", "lib")
		if err := os.MkdirAll(childDir, 0755); err != nil {
			t.Fatalf("failed to create dirs: %v", err)
		}

		configPath := filepath.Join(parentDir, "ribbin.toml")
		if err := os.WriteFile(configPath, []byte("[shims]\n"), 0644); err != nil {
			t.Fatalf("failed to create config: %v", err)
		}

		if err := os.Chdir(childDir); err != nil {
			t.Fatalf("failed to chdir: %v", err)
		}

		found, err := FindProjectConfig()
		if err != nil {
			t.Fatalf("FindProjectConfig error: %v", err)
		}
		if found != configPath {
			t.Errorf("expected %s, got %s", configPath, found)
		}
	})

	t.Run("returns empty string when no config found", func(t *testing.T) {
		// Create a directory with no config anywhere up the tree
		noConfigDir := filepath.Join(tmpDir, "noconfig", "deep", "path")
		if err := os.MkdirAll(noConfigDir, 0755); err != nil {
			t.Fatalf("failed to create dirs: %v", err)
		}

		if err := os.Chdir(noConfigDir); err != nil {
			t.Fatalf("failed to chdir: %v", err)
		}

		found, err := FindProjectConfig()
		if err != nil {
			t.Fatalf("FindProjectConfig error: %v", err)
		}
		if found != "" {
			t.Errorf("expected empty string, got %s", found)
		}
	})
}

func TestLoadProjectConfig(t *testing.T) {
	t.Run("loads valid config", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "ribbin-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		configPath := filepath.Join(tmpDir, "ribbin.toml")
		content := `[shims.cat]
action = "block"
message = "Use bat instead"
paths = ["/bin/cat", "/usr/bin/cat"]
`
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		cfg, err := LoadProjectConfig(configPath)
		if err != nil {
			t.Fatalf("LoadProjectConfig error: %v", err)
		}

		if cfg.Shims == nil {
			t.Fatal("Shims map is nil")
		}

		catShim, exists := cfg.Shims["cat"]
		if !exists {
			t.Fatal("cat shim not found")
		}
		if catShim.Action != "block" {
			t.Errorf("expected action 'block', got '%s'", catShim.Action)
		}
		if catShim.Message != "Use bat instead" {
			t.Errorf("unexpected message: %s", catShim.Message)
		}
		if len(catShim.Paths) != 2 {
			t.Errorf("expected 2 paths, got %d", len(catShim.Paths))
		}
	})

	t.Run("returns error for missing file", func(t *testing.T) {
		_, err := LoadProjectConfig("/nonexistent/path/ribbin.toml")
		if err == nil {
			t.Error("expected error for missing file")
		}
	})

	t.Run("returns error for invalid TOML", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "ribbin-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		configPath := filepath.Join(tmpDir, "ribbin.toml")
		// Invalid TOML - unquoted string value
		content := `[shims.cat]
action = unquoted
`
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		_, err = LoadProjectConfig(configPath)
		if err == nil {
			t.Error("expected error for invalid TOML")
		}
	})

	t.Run("handles empty shims section", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "ribbin-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		configPath := filepath.Join(tmpDir, "ribbin.toml")
		content := `[shims]
`
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		cfg, err := LoadProjectConfig(configPath)
		if err != nil {
			t.Fatalf("LoadProjectConfig error: %v", err)
		}

		if len(cfg.Shims) != 0 {
			t.Errorf("expected empty shims, got %d", len(cfg.Shims))
		}
	})

	t.Run("loads config with passthrough", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "ribbin-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		configPath := filepath.Join(tmpDir, "ribbin.toml")
		content := `[shims.tsc]
action = "block"
message = "Use pnpm run typecheck"
passthrough = { invocation = ["pnpm run"], invocationRegexp = ["pnpm (typecheck|build)"] }
`
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		cfg, err := LoadProjectConfig(configPath)
		if err != nil {
			t.Fatalf("LoadProjectConfig error: %v", err)
		}

		tscShim, exists := cfg.Shims["tsc"]
		if !exists {
			t.Fatal("tsc shim not found")
		}
		if tscShim.Action != "block" {
			t.Errorf("expected action 'block', got '%s'", tscShim.Action)
		}
		if tscShim.Passthrough == nil {
			t.Fatal("passthrough config is nil")
		}
		if len(tscShim.Passthrough.Invocation) != 1 {
			t.Errorf("expected 1 invocation pattern, got %d", len(tscShim.Passthrough.Invocation))
		}
		if tscShim.Passthrough.Invocation[0] != "pnpm run" {
			t.Errorf("unexpected invocation pattern: %s", tscShim.Passthrough.Invocation[0])
		}
		if len(tscShim.Passthrough.InvocationRegexp) != 1 {
			t.Errorf("expected 1 invocationRegexp pattern, got %d", len(tscShim.Passthrough.InvocationRegexp))
		}
		if tscShim.Passthrough.InvocationRegexp[0] != "pnpm (typecheck|build)" {
			t.Errorf("unexpected invocationRegexp pattern: %s", tscShim.Passthrough.InvocationRegexp[0])
		}
	})

	t.Run("loads config with scopes", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "ribbin-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		configPath := filepath.Join(tmpDir, "ribbin.toml")
		content := `[shims.cat]
action = "block"
message = "Use bat"

[scopes.frontend]
path = "apps/frontend"
extends = ["root"]

[scopes.frontend.shims.npm]
action = "block"
message = "Use pnpm in frontend"

[scopes.backend]
path = "apps/backend"

[scopes.backend.shims.yarn]
action = "block"
message = "Use npm in backend"
`
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		cfg, err := LoadProjectConfig(configPath)
		if err != nil {
			t.Fatalf("LoadProjectConfig error: %v", err)
		}

		// Check root shim
		if _, exists := cfg.Shims["cat"]; !exists {
			t.Error("root cat shim not found")
		}

		// Check scopes
		if cfg.Scopes == nil {
			t.Fatal("Scopes map is nil")
		}
		if len(cfg.Scopes) != 2 {
			t.Errorf("expected 2 scopes, got %d", len(cfg.Scopes))
		}

		// Check frontend scope
		frontend, exists := cfg.Scopes["frontend"]
		if !exists {
			t.Fatal("frontend scope not found")
		}
		if frontend.Path != "apps/frontend" {
			t.Errorf("expected path 'apps/frontend', got '%s'", frontend.Path)
		}
		if len(frontend.Extends) != 1 || frontend.Extends[0] != "root" {
			t.Errorf("unexpected extends: %v", frontend.Extends)
		}
		if _, exists := frontend.Shims["npm"]; !exists {
			t.Error("frontend npm shim not found")
		}

		// Check backend scope
		backend, exists := cfg.Scopes["backend"]
		if !exists {
			t.Fatal("backend scope not found")
		}
		if backend.Path != "apps/backend" {
			t.Errorf("expected path 'apps/backend', got '%s'", backend.Path)
		}
		if len(backend.Extends) != 0 {
			t.Errorf("expected no extends, got %v", backend.Extends)
		}
	})

	t.Run("rejects scope with parent traversal in path", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "ribbin-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		configPath := filepath.Join(tmpDir, "ribbin.toml")
		content := `[scopes.escape]
path = "../outside"

[scopes.escape.shims.bad]
action = "block"
`
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		_, err = LoadProjectConfig(configPath)
		if err == nil {
			t.Error("expected error for path with parent traversal")
		}
	})

	t.Run("accepts scope with empty path (defaults to .)", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "ribbin-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		configPath := filepath.Join(tmpDir, "ribbin.toml")
		content := `[scopes.mixin]

[scopes.mixin.shims.rm]
action = "block"
message = "Use trash"
`
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		cfg, err := LoadProjectConfig(configPath)
		if err != nil {
			t.Fatalf("LoadProjectConfig error: %v", err)
		}

		mixin, exists := cfg.Scopes["mixin"]
		if !exists {
			t.Fatal("mixin scope not found")
		}
		if mixin.Path != "" {
			t.Errorf("expected empty path, got '%s'", mixin.Path)
		}
	})
}

func TestValidateScopePath(t *testing.T) {
	t.Run("accepts empty path", func(t *testing.T) {
		err := ValidateScopePath("", "/some/dir")
		if err != nil {
			t.Errorf("unexpected error for empty path: %v", err)
		}
	})

	t.Run("accepts dot path", func(t *testing.T) {
		err := ValidateScopePath(".", "/some/dir")
		if err != nil {
			t.Errorf("unexpected error for dot path: %v", err)
		}
	})

	t.Run("accepts relative descendant path", func(t *testing.T) {
		err := ValidateScopePath("apps/frontend", "/some/dir")
		if err != nil {
			t.Errorf("unexpected error for relative path: %v", err)
		}
	})

	t.Run("accepts nested relative path", func(t *testing.T) {
		err := ValidateScopePath("apps/frontend/src/components", "/some/dir")
		if err != nil {
			t.Errorf("unexpected error for nested path: %v", err)
		}
	})

	t.Run("rejects parent traversal at start", func(t *testing.T) {
		err := ValidateScopePath("../outside", "/some/dir")
		if err == nil {
			t.Error("expected error for parent traversal at start")
		}
	})

	t.Run("rejects parent traversal in middle", func(t *testing.T) {
		err := ValidateScopePath("apps/../../../outside", "/some/dir")
		if err == nil {
			t.Error("expected error for parent traversal in middle")
		}
	})

	t.Run("rejects pure parent traversal", func(t *testing.T) {
		err := ValidateScopePath("..", "/some/dir")
		if err == nil {
			t.Error("expected error for pure parent traversal")
		}
	})

	t.Run("accepts absolute path under config dir", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "ribbin-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		absPath := filepath.Join(tmpDir, "apps", "frontend")
		err = ValidateScopePath(absPath, tmpDir)
		if err != nil {
			t.Errorf("unexpected error for absolute path under config dir: %v", err)
		}
	})

	t.Run("rejects absolute path outside config dir", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "ribbin-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		configDir := filepath.Join(tmpDir, "project")
		outsidePath := filepath.Join(tmpDir, "other")

		err = ValidateScopePath(outsidePath, configDir)
		if err == nil {
			t.Error("expected error for absolute path outside config dir")
		}
	})
}
