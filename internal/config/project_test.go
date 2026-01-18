package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindProjectConfig(t *testing.T) {
	// Create a temp directory structure
	tmpDir, err := os.MkdirTemp("", "ribbin-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

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
}
